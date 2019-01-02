package recvs

import (
	"time"

	"github.com/Laisky/go-fluentd/libs"
	"github.com/Laisky/go-syslog"
	"github.com/Laisky/go-utils"
	"go.uber.org/zap"
)

func NewRsyslogSrv(addr string) (*syslog.Server, syslog.LogPartsChannel) {
	inchan := make(syslog.LogPartsChannel, 1000)
	handler := syslog.NewChannelHandler(inchan)

	server := syslog.NewServer()
	server.SetFormat(syslog.Automatic)
	server.SetHandler(handler)
	server.ListenUDP(addr)
	server.ListenTCP(addr)
	return server, inchan
}

type RsyslogCfg struct {
	Addr, Env, TagKey string
}

// RsyslogRecv
type RsyslogRecv struct {
	*BaseRecv
	*RsyslogCfg
}

func NewRsyslogRecv(cfg *RsyslogCfg) *RsyslogRecv {
	return &RsyslogRecv{
		BaseRecv:   &BaseRecv{},
		RsyslogCfg: cfg,
	}
}

func (r *RsyslogRecv) GetName() string {
	return "RsyslogRecv"
}

func (r *RsyslogRecv) Run() {
	utils.Logger.Info("Run RsyslogRecv")

	go func() {
		defer utils.Logger.Panic("rsyslog reciver exit", zap.String("name", r.GetName()))
		var (
			err error
			msg *libs.FluentMsg
			tag = "emqtt." + r.Env
		)
		for {
			srv, inchan := NewRsyslogSrv(r.Addr)
			utils.Logger.Info("listening rsyslog", zap.String("addr", r.Addr))
			if err = srv.Boot(&syslog.BLBCfg{
				ACK: []byte{},
				SYN: "hello",
			}); err != nil {
				utils.Logger.Error("try to start rsyslog server got error", zap.Error(err))
				continue
			}

			for logPart := range inchan {
				switch logPart["timestamp"].(type) {
				case time.Time:
					logPart["@timestamp"] = logPart["timestamp"].(time.Time).UTC().Format("2006-01-02T15:04:05.000000Z")
					delete(logPart, "timestamp")
				default:
					utils.Logger.Error("unknown timestamp format")
				}

				// rename to message because of the elasticsearch default query field is `message`
				logPart["message"] = logPart["content"]
				delete(logPart, "content")

				msg = r.msgPool.Get().(*libs.FluentMsg)
				// utils.Logger.Info(fmt.Sprintf("got %p", msg))
				msg.Id = r.counter.Count()
				msg.Tag = tag
				msg.Message = logPart
				msg.Message[r.TagKey] = msg.Tag

				r.asyncOutChan <- msg
			}

			if err = srv.Kill(); err != nil {
				utils.Logger.Error("stop rsyslog got error", zap.Error(err))
			}
		}
	}()
}
