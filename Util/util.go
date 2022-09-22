package Util

import (
	"fmt"
	"github.com/go-ping/ping"
	"golang.org/x/net/context"
	"gopkg.in/gomail.v2"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"
)

// DetectPort 检测端口状态
func DetectPort(ipPort string, timeout time.Duration) (bool, string, error) {
	conn, err := net.DialTimeout("tcp", ipPort, timeout)
	if err != nil {
		return false, "Dial Error", err
	}
	err = conn.Close()
	if err != nil {
		return false, "Connect Close Error", err
	}
	return true, "Port Open", nil
}

// DetectPing Ping目标主机，返回平均rtt
func DetectPing(host string, timeout time.Duration) (bool, string, error) {
	pinger, err := ping.NewPinger(host)
	if err != nil {
		return false, "New Pinger Error", err
	}
	pinger.SetPrivileged(true) //设置权限
	pinger.Count = 4           // 发送n个包
	pinger.Timeout = timeout   // 1000ms超时
	err = pinger.Run()         // Blocks until finished.
	if err != nil {
		return false, "Pinger Run Error", err
	}
	stats := pinger.Statistics()
	if stats.PacketsRecv == 0 {
		return false, "Time Out, Loss 100%", err
	}
	info := fmt.Sprintf("AvgRtt:%dms, Loss %.0f%%", stats.AvgRtt.Milliseconds(), stats.PacketLoss)
	return true, info, err
}

// DetectGetTimeout 检测网页或者Get接口
func DetectGetTimeout(url string, timeout time.Duration) (bool, string, error) {
	var httpClientTimeout = &http.Client{ // *http.Client
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, netw, addr string) (net.Conn, error) {
				conn, err := net.DialTimeout(netw, addr, timeout)
				if err != nil {
					return nil, err
				}
				return conn, nil

			},
			MaxIdleConnsPerHost:   10,
			ResponseHeaderTimeout: timeout,
		},
	}
	_, err := httpClientTimeout.Get(url)
	if err != nil {
		return false, "Get Error", err
	}
	return true, "Alive", err
}

func CtrlCExitWithFunc(exitFunc func()) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	func() {
		for range c {
			exitFunc()
			return
		}
	}()
}

func CtrlCExit() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	func() {
		for range c {
			return
		}
	}()
}

func SendEmail(fromAddr, sendName, toAddr, subject, contextBody, hostAddr, authCode string, hostPort int) error {
	m := gomail.NewMessage()
	m.SetHeader("From", m.FormatAddress(fromAddr, sendName))
	m.SetHeader("To", toAddr)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", contextBody)
	d := gomail.NewDialer(hostAddr, hostPort, fromAddr, authCode)
	if err := d.DialAndSend(m); err != nil {
		return err
	}
	return nil
}
