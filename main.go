package main

import (
	"encoding/json"
	"flag"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"utilens/vlog"
	"path/filepath"
	"os/user"
	"github.com/robfig/cron"
	"errors"
)

const (
	T_D = "dir"
	T_F = "file"
)

var lcron = cron.New()

var confMode ConfModel

var defaultConfDirs = extDefaultConfDir([]string{
	"/etc/logpack",
})

type ConfModel struct {
	typ string
	path string
}



func init() {
	var confdir string
	var conffile string


	//加载配置文件， 默认加载路径 /etc/logpack、 当前用户工作空间 ~/etc/logpack 如果通过 -f 指定那么会只加载指定的配置文件，以上配置文件目录会被忽略
	flag.StringVar(&confdir, "d", "", "指定配置文件路径，默认会地柜加载指定路径所有文件")
	flag.StringVar(&conffile, "f", "", "指定唯一的配置文件。 如果指定 -f 那么-d将会忽略")
	flag.IntVar(&compressRate,"rate", 9, "自定压缩比率")
	if "" == conffile { //加载文件夹
		if "" != confdir {
			confMode = ConfModel{
				typ : T_D,
				path: confdir,
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
			typ:T_F,
			path:conffile,
		}
		return
	}

}

//扩展默认加载配置文件的目录支持家目录加载
func extDefaultConfDir(defaultConfDirs []string) []string{
	user,err := user.Current()
	if err != nil{
		vlog.Error("获取用户家目录失败, 将忽略默认家目录配置文件加载")
		return defaultConfDirs
	}
	extConfdirs := defaultConfDirs
	for _,d := range defaultConfDirs{
		extConfdirs = append(extConfdirs,filepath.Join(user.HomeDir,d))
	}
	return extConfdirs
}


// 加载目录， 递归加载
func loadDir(d string) ([]*Conf, error) {
	confs := make([]*Conf,0)
	f, err := os.Stat(d)
	if (! os.IsNotExist(err)) && f.IsDir() {
		files, err := ioutil.ReadDir(d)
		if err != nil {
			vlog.Info("dir load fail the path is : ", f.Name() , err)
			return nil,err
		}
		if len(files) == 0 {
			vlog.Debug("the path not config file:",d)
			return nil,errors.New("the path is empty: " + d)
		}
		for _, f := range files {
			if f.IsDir() {
				p := filepath.Join(d,f.Name())
				cs,err := loadDir(p);
				if err == nil{
					confs = append(confs,cs...)
					continue
				}else{
					vlog.Error("dir load fail",err)
					continue
				}
			}else{
				c,err := loadConfile(filepath.Join(d,f.Name()))
				if err != nil{
					vlog.Error("config file load failed",err)
					continue
				}else{
					confs = append(confs,c)
					continue
				}
			}
		}
	} else {
		vlog.Debug("path: " + d + " not exist")
		return nil,err
	}
	return confs,err
}

// 加载配置文件，到 confs
func loadConfile(fl string) (*Conf,error ){
	conf := &Conf{}
	bt,err := ioutil.ReadFile(fl)
	if err != nil {
		vlog.Error("the path " + fl + " load fail")
		return nil,err
	} else {
		if err := yaml.Unmarshal(bt, &conf); err != nil {
			vlog.Error(" path : " + fl + "解析失败 ", err)
			return nil,err
		}else{
			if validateConf(conf) {
				return conf,nil
			}else{
				return nil,errors.New("文件检验失败: "+fl)
			}
		}

	}
}

//校验配置文件，archive 必填项 schedule和dirs  logrotate必填项为schedule和files
func validateConf(conf *Conf) bool{

	if conf.Archives != nil{
		for i,a := range conf.Archives{
			if empty(a.Schedule) || empty(a.Dirs) || len(a.Dirs) == 0{
				bt,_ := json.Marshal(a)
				vlog.Error("invalidate archive config , schedule and dirs by need ",string(bt))
				conf.Archives = append(conf.Archives[:i],conf.Archives[i+1:]...)
			}
		}
		if len(conf.Archives) == 0{
			conf.Archives = nil
		}
	}
	if conf.Logrotates != nil{
		for i,v:= range conf.Logrotates {
			if empty(v.Schedule) || empty(v.Files) || len(v.Files) == 0 {
				bt,_ := json.Marshal(v)
				vlog.Error("invalidate logrotate config ,schedule and files by need" , string(bt))
				conf.Logrotates = append(conf.Logrotates[:i], conf.Logrotates[i+1:]...)
			}
		}
		if (len(conf.Logrotates) == 0 ){
			conf.Logrotates = nil
		}
	}

	if conf.Archives == nil && conf.Logrotates == nil{
		return false
	}
	return true
}

func empty(v interface{}) bool  {
	if(v == nil){
		return true
	}
	if k,ok := v.(string); ok{
		return "" == k
	}

	return false
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

func start()  {
	confs := make([]*Conf,0)
	if confMode.typ == T_F{
		conf,err := loadConfile(confMode.path)
		if err != nil{
			vlog.Error("配置文件加载失败",confMode.path,err)
			return
		}else{
			confs = append(confs,conf)
		}
	}else if confMode.typ == T_D{
		cs,err := loadDir(confMode.path)
		if err != nil{
			vlog.Error("配置文件加载失败",confMode.path,err)
			return
		}else{
			confs = append(confs,cs...)
		}
	}
	if len(confs) >0{
		restartCron(confs)
	}else{
		vlog.Error("cron 启动失败")
	}
}


func main() {

	start()

	select {

	}

}
