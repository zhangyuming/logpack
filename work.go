package main

import (
	"utilens/vlog"
	"encoding/json"
	"time"
	"fmt"
	"os"
	"io"
	"compress/gzip"
	"io/ioutil"
	"strings"
	"strconv"
	"path/filepath"
	"sort"
	"sync"
	"archive/tar"
	"os/exec"
	)

var compressRate int = 9
var defaultRotateFileSize string = "1M"
var defaultArchivePrevious int = 0
var defaultArchiveDir = "logpack"

var lockMap = make(map[string]chan int,5)
var down = make(chan int,0)


const (
	logrotateSuffix = "-logpack-"
	archiveDir = "logpack"
)

type Conf struct {
	Name       string      `yaml:"name"`
	Logrotates []*Logrotate `yaml:"logrotate"`
	Archives   []*Archive   `yaml:"archive"`
}

// 公共的config 实体
type Logrotate struct {
	sync.Mutex
	Name     string   `yaml:"name"`
	Schedule string   `yaml:"schedule"`
	Rotate   int      `yaml:"rotate"`
	Files    []string `yaml:"files"`
	Size     string   `yaml:"size"`
	Compress bool     `yaml:"compress"`
}

type Archive struct {
	sync.Mutex
	Name     string   `yaml:"name"`
	Schedule string   `yaml:"schedule"`
	Rotate   int      `yaml:"rotate"`
	Previous   int      `yaml:"previous"`
	Dirs     []string `yaml:"dirs"`
}

func (c Logrotate)String()string  {
	bt,err := json.Marshal(c)
	if(err != nil){
		vlog.Info("logrotate 格式化失败",err)
	}
	return string(bt)
}

func (c Archive)String()string  {
	bt,err := json.Marshal(c)
	if(err != nil){
		vlog.Info("archive 格式化失败",err)
	}
	return string(bt)
}

// 轮替文件
func rotateFile(name string) (newName string,err error){

	//dir := filepath.Dir(name)
	newName = fmt.Sprint(name,"-logpack-",time.Now().Unix())

	f,err := os.Open(name)
	if err != nil{
		vlog.Error("open file failed ", name)
		return
	}
	defer f.Close()
	t,err := os.OpenFile(newName, os.O_CREATE|os.O_WRONLY,0666)
	if err != nil{
		vlog.Error("create file faild ", newName)
		return
	}
	defer t.Close()
	_,err = io.Copy(t,f)
	f.Close()
	if err != nil{
		vlog.Error("copy file failed: source ", name ,"  to: ", newName ,err)
		return
	}
	t.Sync()
	t.Close()
	err = os.Truncate(name,0)
	if err != nil{
		vlog.Debug("清空文件失败",err)
	}
	return newName, nil
}

//比较文件大小
func compareFileSize( size string ,fileInfo os.FileInfo) (result int, err error) {

	var ts int64
	var unit string
	if empty(size){
		size = defaultRotateFileSize
	}
	if strings.ContainsAny(size,"GgMmKk"){
		unit = size[len(size)-1:]
		ts,err = strconv.ParseInt(size[:len(size)-1],10,64)
		if err != nil{
			vlog.Error("转化文件大小失败",size)
			return 0,err
		}

	}else{
		ts,err = strconv.ParseInt(size,10,64)
		if err != nil{
			vlog.Error("转化文件大小失败",size)
			return 0,err
		}
	}
	switch unit {
	case "G","g":
		ts = ts * 1024 * 1024 * 1024
	case "M","m":
		ts = ts * 1024 * 1024
	case "K","k":
		ts = ts * 1024
	default:

	}
	vlog.Debug("期望值 ",ts," 实际值" , fileInfo.Size())
	if fileInfo.Size() > ts {
		return 1,nil
	}else{
		return -1,nil
	}

}

// 压缩文件
func zipFile(name string) error{
	in,err := os.Open(name)
	if err != nil{
		vlog.Error("open file failed" , name , err)
		return err
	}
	defer in.Close()
	out,err := os.OpenFile(fmt.Sprint(name,".gz"),os.O_CREATE|os.O_WRONLY,0666)
	if err != nil {
		vlog.Error("create file failed ", fmt.Sprint(name,".gz"),err)
	}
	defer out.Close()
	var w *gzip.Writer
	if w != nil{
		defer w.Close()
	}
	switch compressRate {
	case gzip.BestCompression:
		w,err = gzip.NewWriterLevel(out,gzip.BestCompression)
		if err != nil{
			vlog.Debug("创建压缩文件失败,使用最大压缩比",err)
		}
	case gzip.BestSpeed:
		w,err = gzip.NewWriterLevel(out,gzip.BestSpeed)
		if err != nil{
			vlog.Debug("创建压缩文件失败,使用最大压缩比",err)
		}
	case gzip.DefaultCompression:
		w,err = gzip.NewWriterLevel(out,gzip.DefaultCompression)
		if err != nil{
			vlog.Debug("创建压缩文件失败,使用最大压缩比",err)
		}
	case gzip.HuffmanOnly:
		w,err = gzip.NewWriterLevel(out,gzip.HuffmanOnly)
		if err != nil{
			vlog.Debug("创建压缩文件失败,使用最大压缩比",err)
		}
	case gzip.NoCompression:
		w,err = gzip.NewWriterLevel(out,gzip.NoCompression)
		if err != nil{
			vlog.Debug("创建压缩文件失败,使用最大压缩比",err)
		}
	default:
		w,err = gzip.NewWriterLevel(out,gzip.BestCompression)
	}
	bt,err := ioutil.ReadAll(in)
	if err != nil{
		vlog.Error("读取文件失败",name, err)
		return err
	}
	if _,err = w.Write(bt); err != nil {
		vlog.Error("压缩文件失败")
		return err
	}
	w.Flush()
	w.Close()
	return nil
}

// 获取文件夹中未被程序打开的文件，并且文件修改时间大于 previous天的文件
func listUnUsedFiles(dir string, previous int) ( files []*string, err error){


	if _,p := filepath.Split(dir); p == "logpack"{
		vlog.Debug("logpack 文件夹不参与打包压缩")
		return []*string{},nil
	}

	fis,err := ioutil.ReadDir(dir)
	if err != nil{
		return nil, err
	}
	for _,fi := range fis{
		if fi.IsDir(){
			fs,err := listUnUsedFiles(filepath.Join(dir,fi.Name()),previous)
			if(err != nil){
				vlog.Error("获取文件列表失败",dir,err)
			}else{
				files = append(files,fs...)
				continue
			}
		}
		ft := filepath.Join(dir,fi.Name())
		if ! isUsedFile(ft) {
			du := time.Now().Sub(fi.ModTime())
			if du.Hours() > float64(previous*24) {
				vlog.Debug("添加文件：",ft,"到待压缩列表")
				files = append(files,&ft)
			}else {
				vlog.Debug(ft,"在指定使时间范围内不能压缩, 时间为",previous,"天")
			}
		}else {
			vlog.Debug(ft," 被使用中 不能压缩该文件")
		}
	}
	return files,nil
}

// 打包文件
func tarFiles(files []*string, tarName string, deleteSource bool)(error){

	f,err := os.OpenFile(tarName,os.O_CREATE|os.O_WRONLY,0666)
	if err != nil{
		vlog.Error("创建压缩包文件失败",err)
		return err
	}
	defer f.Close()
	tarw := tar.NewWriter(f)
	defer tarw.Close()
	//wg := sync.WaitGroup{}
	for _,fpath := range files{
		//wg.Add(1)
		func(f *string) {
			//defer wg.Done()
			fl,err := os.Open(*f)
			if err != nil{
				vlog.Error("打开文件失败",err)
				return
			}
			defer fl.Close()
			finfo,err := fl.Stat()
			if err != nil{
				vlog.Error("获取文件详情失败",err)
				return
			}
			header,err := tar.FileInfoHeader(finfo,"")
			if err != nil{
				vlog.Error("获取tar包header 失败",err)
				return
			}
			if err = tarw.WriteHeader(header); err != nil{
				vlog.Error("打包文件header 失败",err)
				return
			}
			if _,err = io.Copy(tarw,fl); err != nil{
				vlog.Error("文件打包失败",*f,err)
				return
			}
			fl.Close()
			tarw.Flush()
			vlog.Debug("打包文件",*f," 成功")
			if deleteSource{
				vlog.Debug("删除源文件",*f)
				if err := os.Remove(*f); err != nil{
					vlog.Error("删除源文件失败",err)
				}
			}
		}(fpath)
	}
	tarw.Flush()
	tarw.Close()

	//wg.Wait()
	return nil
}


// 判断文件是在使用中
func isUsedFile(file string) bool{

	lsof := exec.Command("lsof",file)
	lsofout,_ := lsof.StdoutPipe()
	lsof.Start()

	wc := exec.Command("wc","-l")
	wc.Stdin = lsofout
	out,err:= wc.Output()
	if err != nil{
		vlog.Error("lsof file failed : ",file)
		return true
	}
	rslt,err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil{
		vlog.Error("lsof result convert to int exception, result is :", string(out))
		return true
	}
	if rslt == 0{
		vlog.Debug("file is unused: ",file)
		return false
	}else{
		vlog.Debug("file is used ",file)
		return true
	}



}

func (c *Logrotate) Run() {
	c.Lock()
	bt,_:= json.Marshal(c)
	vlog.Debug("获得对象锁",string(bt))

	defer func() {
		c.Unlock()
		vlog.Debug("释放对象锁",string(bt))
	}()

	vlog.Info("==========================================logrotate begin run ",c)
	wg := sync.WaitGroup{}
	for _,fname := range c.Files{
		wg.Add(1)
		go func(f string){
		//func(f string){
			defer wg.Done()
			vlog.Debug("rotate file : ", f)
			fl,err := os.Open(f)
			if err != nil{
				vlog.Error("open file failed",err)
				return
			}
			defer fl.Close()
			finfo,err := fl.Stat();
			if err != nil{
				vlog.Error("fileinfo failed",err)
				return
			}
			r,err := compareFileSize(c.Size,finfo)
			if err != nil{
				vlog.Error("比较文件失败",err)
				return
			}
			if r ==1 {
				nf,err := rotateFile(f)
				if err != nil{
					vlog.Error("轮替文件失败",f,err)
					return
				}
				fl.Close()
				if c.Compress{
					vlog.Debug("压缩文件",nf)
					if err = zipFile(nf); err == nil{
						vlog.Debug("压缩文件成功")
						os.Remove(nf)
					}else{
						vlog.Debug("压缩文件失败")
					}
				}
			}else{
				vlog.Debug("文件大小小于预设值或者小于默认值",defaultRotateFileSize,"无需轮替, 文件：",f," size:",finfo.Size())
			}
		}(fname)
	}
	wg.Wait()
	if c.Rotate > 0{
		for _,f := range c.Files{
			var handlerFiles []os.FileInfo
			fis,err := ioutil.ReadDir(filepath.Dir(f))
			if err != nil{
				vlog.Error("读取文件夹失败",err)
			}
			filePath,filename := filepath.Split(f)
			for _,finfo := range fis{
				if strings.HasPrefix(finfo.Name(),filename){
					handlerFiles = append(handlerFiles,finfo)
				}
			}

			if len(handlerFiles) > c.Rotate{
				sort.Slice(handlerFiles, func(i, j int) bool {
					return handlerFiles[i].ModTime().After(handlerFiles[j].ModTime())
				})
				if len(handlerFiles) <= c.Rotate{
					vlog.Debug(f,"轮替文件小于指定的数量",len(handlerFiles),c.Rotate)
					continue
				}
				for _,fi := range handlerFiles[c.Rotate+1:]{
					fabs := filepath.Join(filePath,fi.Name())
					vlog.Debug("删除过期的压缩文件",fabs)
					if e:= os.Remove(fabs); e != nil{
						vlog.Error("删除文件是失败",e)
					}
				}
			}
		}
	}
	vlog.Info("==========================================logrotate begin end ")

}

func (c *Archive) Run() {
	c.Lock()
	bt,_:= json.Marshal(c)
	vlog.Debug("获得对象锁",string(bt))
	defer func() {
		c.Unlock()
		vlog.Debug("释放对象锁",string(bt))
	}()

	vlog.Info("****************************************** archive begin",c)
	wg := sync.WaitGroup{}
	for _,d := range c.Dirs{
		wg.Add(1)
		go func(dir string) {
			defer wg.Done()
			fs,err := listUnUsedFiles(dir,c.Previous)
			if err != nil {
				vlog.Error("获取文件夹",dir," 中文件失败",err)
				return
			}
			if len(fs) == 0{
				vlog.Debug("没有待打包的文件")
				return
			}
			logPackDir := filepath.Join(dir,defaultArchiveDir)
			_,err = os.Stat(logPackDir)
			if err != nil{
				vlog.Debug("创建logpack文件夹",logPackDir)
				if err = os.Mkdir(filepath.Join(dir,defaultArchiveDir),0755); err != nil{
					vlog.Error("创建logpack文件夹失败",err)
					return
				}
			}
			tn := filepath.Join(logPackDir,fmt.Sprint("logpack-",time.Now().Unix(),".tar"))
			if err = tarFiles(fs,tn,true); err != nil{
				vlog.Error("打包文件列表失败",fs,err)
				return
			}else{
				if err = zipFile(tn); err != nil{
					vlog.Error("压缩文件失败",tn,err)
					return
				}else{
					vlog.Debug("压缩文件成功",tn)
					if err = os.Remove(tn); err !=nil{
						vlog.Error("删除打包文件失败",err)
					}
				}
			}

			if c.Rotate > 0 {
				fis,err := ioutil.ReadDir(logPackDir)
				if err != nil{
					vlog.Error("查看文件夹失败",logPackDir)
					return
				}
				sort.Slice(fis, func(i, j int) bool {
					return fis[i].ModTime().After(fis[j].ModTime())
				})
				if len(fis) <= c.Rotate{
					vlog.Debug(dir,"文件小于制定的文件数量：",len(fis),c.Rotate)
					return
				}
				for _,f := range fis[c.Rotate:]{
					vlog.Debug("删除过期文件",f)
					if err = os.Remove(filepath.Join(logPackDir,f.Name())); err != nil{
						vlog.Error("删除过期文件失败",err)
						return
					}else{
						vlog.Debug("删除过期的文件",filepath.Join(logPackDir,f.Name()))
					}
				}
			}
		}(d)
	}
	wg.Wait()
	vlog.Info("****************************************** archive end")
}
