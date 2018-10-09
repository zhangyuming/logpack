package main

import (
	"gopkg.in/yaml.v2"
	"github.com/zhangyuming/logpack/vlog"
	"encoding/json"
	"io/ioutil"
	"os/user"
	"path/filepath"
	"os"
	"errors"
	"strings"
)

const (
	T_D = "dir"
	T_F = "file"
)

var defaultConfDirs = extDefaultConfDir([]string{
	"/etc/logpack",
})

type ConfModel struct {
	typ string
	path string
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
	if strings.HasSuffix(fl,"yaml") || strings.HasSuffix(fl,"yml"){
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
	} else {
		vlog.Debug("file ",fl,"not endwith yaml or yml , skip file ")
		return nil,errors.New("file ext not yaml or yml: "+fl)
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

