package activity

import (
 "context"
 "time"
)

type NextScan struct { At time.Time `json:"at"`; RivalChannels int `json:"rivalChannels"` }
func NextScanFromTimes(times []time.Time) *NextScan {
 if len(times)==0 {return nil}
 earliest:=times[0];for _,at:=range times[1:] {if at.Before(earliest){earliest=at}}
 slot:=localHour(earliest);count:=0
 for _,at:=range times {if localHour(at).Equal(slot){count++}}
 return &NextScan{At:earliest,RivalChannels:count}
}

func localHour(at time.Time) time.Time {
 local:=at.In(time.Local)
 return time.Date(local.Year(),local.Month(),local.Day(),local.Hour(),0,0,0,local.Location())
}
// SetScanScheduleSource must be called before the HTTP server starts.
func (s *Service) SetScanScheduleSource(source func(context.Context)([]time.Time,error)) {s.scanSchedule=source}
