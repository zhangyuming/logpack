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
)

var confs []*Conf

var defaultConfDirs = []string{
	"/etc/logpack",
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
			loadDir(confdir)
		} else {
			for _, d := range defaultConfDirs {
				loadDir(d)
				if len(confs) >0 {
					vlog.Info("use confi dir:", d)
					break
				}
				// 获取用户家目录下默认目录的配置文件
				user,err := user.Current()
				if(err != nil){
					vlog.Error("获取用户信息失败",err)
				}
				hc := filepath.Join(user.HomeDir,d)
				loadDir(hc)
				if len(confs) >0 {
					vlog.Info("use confi dir:", hc)
					break
				}
			}
		}
	} else { //加载指定的文件
		loadConfile(conffile)
	}

}

// 加载目录， 递归加载
func loadDir(d string) {
	f, err := os.Stat(d)
	if (! os.IsNotExist(err)) && f.IsDir() {
		files, err := ioutil.ReadDir(d)
		if err != nil {
			vlog.Info("dir load fail the path is : ", f.Name() , err)
			return
		}
		if len(files) == 0 {
			vlog.Debug("the path not config file:",d)
		}
		for _, f := range files {
			if f.IsDir() {
				p := filepath.Join(d,f.Name())
				loadDir(p)
			}else{
				loadConfile(filepath.Join(d,f.Name()))
			}
		}
	} else {
		vlog.Debug("path: " + d + " not exist")
	}
}

// 加载配置文件，到 confs
func loadConfile(fl string) {
	conf := &Conf{}
	bt,err := ioutil.ReadFile(fl)
	if err != nil {
		vlog.Error("the path " + fl + " load fail")
	} else {
		if err := yaml.Unmarshal(bt, conf); err != nil {
			vlog.Error(" path : " + fl + "解析失败 ", err)
		}else{
			if validateConf(conf) {
				confs = append(confs, conf)
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

func main() {
	if len(confs) ==0 {
		vlog.Error("not find config file, default load dir is ",defaultConfDirs, "and homedir: ",defaultConfDirs)
		os.Exit(2)
	}
	bt, err := json.Marshal(confs)
	if err != nil {
		panic(err)
	}
	vlog.Debug("configs is : ",string(bt))



	cron := cron.New()
	for _,c := range confs{
		if c.Logrotates != nil {
			for _,l := range c.Logrotates{
				bt,err := json.Marshal(l)
				if(err != nil){
					vlog.Debug("json 解析失败",l,err)
				}
				vlog.Info("add logratate to cron ", string(bt))
				cron.AddJob(l.Schedule,l)

			}
		}
		if c.Archives != nil{
			for _,a := range c.Archives{
				bt,err := json.Marshal(a)
				if(err != nil){
					vlog.Debug("json 解析失败",a,err)
				}
				vlog.Info("add archive to cron",string(bt))
				cron.AddJob(a.Schedule,a)
			}
		}
	}
	cron.Run()


}
