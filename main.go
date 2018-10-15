package main

import (
	"encoding/json"
	"flag"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"github.com/zhangyuming/logpack/vlog"
	"github.com/robfig/cron"
	"errors"
	"fmt"
	"path/filepath"
	"os"
	"log"
	"os/exec"
	"os/user"
)

var lcron = cron.New()

var confMode ConfModel

var confdirFlag string
var conffileFlag string
var validateConfFlag bool
var CurrentDIR string
var IsDebug bool
var daemon bool
var logfrontend bool
func init() {

	//加载配置文件， 默认加载路径 /etc/logpack、 当前用户工作空间 ~/etc/logpack 如果通过 -f 指定那么会只加载指定的配置文件，以上配置文件目录会被忽略
	flag.StringVar(&confdirFlag, "d", "", "指定配置文件路径，默认会加载指定路径所有文件")
	flag.StringVar(&conffileFlag, "f", "", "指定唯一的配置文件。 如果指定 -f 那么-d将会忽略")
	flag.IntVar(&compressRate,"rate", 9, "自定压缩比率")
	flag.BoolVar(&validateConfFlag,"t",false,"校验配置文件")
	flag.BoolVar(&IsDebug,"vv",false,"日志级别")
	flag.BoolVar(&daemon,"daemon",false,"后台启动")
	flag.BoolVar(&logfrontend,"e",true,"日志前台显示")
	flag.Parse()


	//后台启动
	if daemon{
		args := os.Args[1:]
		flage := false
		i := 0
		for ; i < len(args); i++ {
			if args[i] == "-daemon" || args[i] == "-daemon=true" || args[i] == "--daemon" || args[i] == "--daemon=true" {
				args[i] = "-daemon=false"
				continue
			}
			if args[i] == "-e" || args[i] == "-e=true"{
				args[i] = "-e=false"
				flage = true
				continue
			}
		}
		if !flage{
			args = append(args, "-e=false")
		}


		f,err := createLogfile()
		if err != nil{
			fmt.Print("create log file error", err)
			os.Exit(1)
		}
		fmt.Println("[logfile is] ", f)
		//fmt.Println(args)
		cmd := exec.Command(os.Args[0], args...)
		cmd.Start()
		fmt.Println("[PID]", cmd.Process.Pid)
		os.Exit(0)
	}


	if IsDebug{
		vlog.Level = "DEBUG"
	}
	if "" == conffileFlag { //加载文件夹
		if "" != confdirFlag {
			confMode = ConfModel{
				typ : T_D,
				path: confdirFlag,
			}
			return
		} else {
			for _, d := range defaultConfDirs {

				fis,err := ioutil.ReadDir(d)
				if err != nil{
					vlog.Info("获取文件夹失败",d," 忽略该文件夹")
					continue
				}else if len(fis) > 0{
					vlog.Info("加载配置文件夹",d)
					confMode = ConfModel{
						typ : T_D,
						path : d,
					}
					return
				}else{
					vlog.Info("文件夹为空",d, "忽略该文件夹")
				}
			}
		}
	} else { //加载指定的文件
		confMode = ConfModel{
			typ:  T_F,
			path: conffileFlag,
		}
		return
	}



}


func restartCron(confs []*Conf)  {

	if len(confs) ==0 {
		vlog.Error("confs is empty ")
		return
	}
	bt,err := yaml.Marshal(confs)
	if err != nil {
		vlog.Error("conf 解析失败")
		return
	}else{
		vlog.Debug("configs is : \n",string(bt))
	}

	lcron.Stop()

	for _,c := range confs{
		if c.Logrotates != nil {
			for _,l := range c.Logrotates{
				bt,err := json.Marshal(l)
				if(err != nil){
					vlog.Debug("json 解析失败",l,err)
				}
				vlog.Info("add logratate to cron ", string(bt))
				lcron.AddJob(l.Schedule,l)

			}
		}
		if c.Archives != nil{
			for _,a := range c.Archives{
				bt,err := json.Marshal(a)
				if(err != nil){
					vlog.Debug("json 解析失败",a,err)
				}
				vlog.Info("add archive to cron",string(bt))
				lcron.AddJob(a.Schedule,a)
			}
		}
	}
	lcron.Start()

}

func wrapperConfs() ([]*Conf,error) {
	confs := make([]*Conf,0)
	if confMode.typ == T_F{
		conf,err := loadConfile(confMode.path)
		if err != nil{
			vlog.Error("配置文件加载失败",confMode.path,err)
			return nil,err
		}else{
			confs = append(confs,conf)
			return confs,nil
		}
	}else if confMode.typ == T_D{
		cs,err := loadDir(confMode.path)
		if err != nil{
			vlog.Error("配置文件加载失败",confMode.path,err)
			return nil,err
		}else{
			confs = append(confs,cs...)
			return confs,nil
		}
	}
	return nil,errors.New("wrapper confs failed")
}

//dir of file exist
func pathExists(path string) bool  {
	_, err := os.Stat(path)
	if err == nil{
		return true
	}
	if os.IsNotExist(err){
		return false
	}
	return false
}

// create log file
func createLogfile()(logfile string, err error){
	var logdir = "/var/log/logpack"

	//创建日志文件
	if(!pathExists(logdir)){
		if err := os.Mkdir(logdir,0755); err != nil {
			u,err := user.Current()
			if err != nil{
				log.Fatalln("get user failed")
				return "",err
			}
			logdir = filepath.Join(u.HomeDir,"logpack")
			if !pathExists(logdir){
				if err = os.Mkdir(logdir,0755); err != nil{
					log.Fatalln("create user home logpack dir fail")
					return "",err
				}
			}

		}
	}

	logfile = filepath.Join(logdir,"logpack.log")
	_,err = os.Create(logfile)
	if err != nil{
		log.Fatalln("create file error", err)
		return
	}
	return logfile,nil
}

func main() {

	confs,err := wrapperConfs()

	if logfrontend{
		vlog.SetLogAppender(os.Stdout)

	}else{
		logfile,err := createLogfile()
		if err != nil{
			log.Print("create log file error ", err)
			return
		}
		vlog.SetLogOut(logfile)
		defaultConf := Conf{}
		defaultConf.Name = "logpack log"
		logrotats := []*Logrotate{}
		logrotats = append(logrotats,&Logrotate{
			Name: "logpack self log",
			Rotate: 5,
			Compress: true,
			Files: []string{logfile},
			Schedule: "0 0 6 * *",
		})
		defaultConf.Logrotates = logrotats
		confs = append(confs,&defaultConf)
	}


	if err != nil || len(confs) ==0 {
		fmt.Println("config load failed")
		return
	}else if validateConfFlag{
		return
	}

	if ! prepare() {
		return
	}


	restartCron(confs)

	select {

	}

}

func prepare() bool  {

	_,err := exec.LookPath("lsof")
	if err != nil{
		vlog.Error("the process need lsof commad , no found lsof command", err)
		return false
	}
	return true
}