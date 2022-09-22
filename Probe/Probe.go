package Probe

import (
	"GoMoniter/Util"
	"fmt"
	"github.com/go-ini/ini"
	"log"
	"strings"
	"sync"
	"time"
)

type Probe struct {
	ProbeName            string
	Port                 string
	TimeOut              time.Duration
	DetectIntervalSecond time.Duration
	EmailFlag            bool
	EmailConfig          EmailConfig
	JobNum               int
	JobNames             []string
	JobSlice             []Job
	workMux              sync.Mutex
	workChan             chan int
}

type Job struct {
	Check  string
	Target string
	Status bool
	Info   string
}

type EmailConfig struct {
	EmailTo             string
	EmailHost           string
	EmailHostPort       int
	EmailUser           string
	AuthorizationCode   string
	AliveIntervalSecond time.Duration
}

var DetectFuncMap = map[string]func(target string, timeout time.Duration) (bool, string, error){
	"port": Util.DetectPort,
	"ping": Util.DetectPing,
	"get":  Util.DetectGetTimeout,
}

func NewProbe(filePath string) *Probe {
	var probe Probe
	probe.LoadIni(filePath)
	return &probe
}

func (t *Probe) LoadIni(filePath string) {
	cfg, err := ini.Load(filePath)
	if err != nil {
		log.Fatal("Fail to read file: ", err)
	}
	// Default Section
	t.ProbeName = cfg.Section("").Key("ProbeName").String()
	t.Port = cfg.Section("").Key("Port").String()
	t.TimeOut = time.Duration(cfg.Section("").Key("TimeOut").MustInt(2)) * time.Second
	t.DetectIntervalSecond = time.Duration(cfg.Section("").Key("DetectIntervalSecond").MustInt(0)) * time.Second
	t.EmailFlag = cfg.Section("").Key("Email").MustBool()
	if t.EmailFlag {
		t.EmailConfig.EmailTo = cfg.Section("").Key("EmailTo").String()
		t.EmailConfig.EmailHost = cfg.Section("").Key("EmailHost").String()
		t.EmailConfig.EmailHostPort = cfg.Section("").Key("EmailHostPort").MustInt()
		t.EmailConfig.EmailUser = cfg.Section("").Key("EmailUser").String()
		t.EmailConfig.AuthorizationCode = cfg.Section("").Key("AuthorizationCode").String()
		t.EmailConfig.AliveIntervalSecond = time.Duration(cfg.Section("").Key("AliveIntervalSecond").MustInt()) * time.Second
	}

	// Section
	sections := cfg.Sections()
	t.JobNum = len(sections) - 1
	t.workChan = make(chan int, t.JobNum)
	if t.JobNum == 0 {
		log.Fatal("No Job")
	}
	t.JobNames = cfg.SectionStrings()[1:]
	t.JobSlice = make([]Job, t.JobNum)
	for i, s := range sections {
		if i == 0 {
			continue
		}
		t.JobSlice[i-1].Check = s.Key("check").String()
		t.JobSlice[i-1].Target = s.Key("target").String()
	}
}

func (t *Probe) Start() {
	//log.Printf("JOB Slice: %v\n", t.JobSlice)
	log.Printf("JOB: %v\n", t.JobNames)
	log.Println("==========First Detection==========")
	t.Work()
	if t.EmailFlag {
		t.SendMail(fmt.Sprintf("[GoMoniter-%s] Start Dection", t.ProbeName),
			t.ListJob(make([]int, 0)))
		if t.EmailConfig.AliveIntervalSecond > 0 {
			t.AliveReport()
		}
	}
	t.Schedule()
}

func (t *Probe) Work() []int {
	t.workMux.Lock()
	for i, job := range t.JobSlice {
		go func(i int, job Job) {
			status, info, err := DetectFuncMap[job.Check](job.Target, t.TimeOut)
			t.JobSlice[i].Info = info

			if t.JobSlice[i].Status != status {
				t.JobSlice[i].Status = status
				t.workChan <- i
			} else {
				t.workChan <- -1
			}

			if err != nil {
				log.Printf("[%s] [Info:%s] Error:%s", t.JobNames[i], info, err)
			} else {
				log.Printf("[%s] [Info:%s] %s %s", t.JobNames[i], info, job.Check, job.Target)
			}
		}(i, job)
	}
	var statusChangedJobIndex []int
	for i := 0; i < t.JobNum; i++ {
		jobId := <-t.workChan
		if jobId == -1 {
			continue
		}
		statusChangedJobIndex = append(statusChangedJobIndex, jobId)
	}
	t.workMux.Unlock()
	return statusChangedJobIndex
}

func (t *Probe) Schedule() {
	tick := Util.NewTicker(t.DetectIntervalSecond, func() {
		log.Println("==========Schedule Work==========")
		statusChangedJobIndex := t.Work()
		if len(statusChangedJobIndex) > 0 && t.EmailFlag {
			t.SendMail(fmt.Sprintf("[GoMoniter-%s] Notification of status changes", t.ProbeName),
				t.ListJob(statusChangedJobIndex))
		}
	}, func() {
		if t.EmailFlag {
			t.SendMail(fmt.Sprintf("[GoMoniter-%s] Exit Report", t.ProbeName),
				"Probe Exit.")
		}
		log.Println("==========Schedule Exit==========")
	})
	tick.Start()
	Util.CtrlCExitWithFunc(tick.Close)
}

func (t *Probe) AliveReport() {
	tick := Util.NewTicker(t.EmailConfig.AliveIntervalSecond, func() {
		t.SendMail(fmt.Sprintf("[GoMoniter-%s] Alive Report", t.ProbeName),
			t.ListJob(make([]int, 0)))
	}, func() {
	})
	tick.Start()
}

func (t *Probe) ListJob(l []int) string {
	var builder strings.Builder
	if len(l) == 0 {
		l = make([]int, t.JobNum, t.JobNum)
		for i, _ := range l {
			l[i] = i
		}
	}
	for _, index := range l {
		builder.WriteString("<b>[")
		builder.WriteString(t.JobNames[index])
		builder.WriteString("]</b><br>")
		builder.WriteString("Check: ")
		builder.WriteString(t.JobSlice[index].Check)
		builder.WriteString("<br>")
		builder.WriteString("Target: ")
		builder.WriteString(t.JobSlice[index].Target)
		builder.WriteString("<br>")
		builder.WriteString("Status: ")
		builder.WriteString(t.JobSlice[index].Info)
		builder.WriteString("<br>")
	}
	return builder.String()
}

func (t *Probe) SendMail(subject, contextBody string) {
	t.workMux.Lock()
	err := Util.SendEmail(t.EmailConfig.EmailUser, t.ProbeName, t.EmailConfig.EmailTo,
		subject, contextBody, t.EmailConfig.EmailHost,
		t.EmailConfig.AuthorizationCode, t.EmailConfig.EmailHostPort)
	if err != nil {
		log.Println(err)
	}
	log.Printf("[Email] %s %s", t.EmailConfig.EmailTo, subject)
	t.workMux.Unlock()
}
