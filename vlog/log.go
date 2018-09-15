package vlog

import (
	"log"
	"io"
	"os"
	"path/filepath"
)

var Level string = "DEBUG"

var writer io.Writer

const(
	I = "INFO"
	D = "DEBUG"
	E = "ERROR"
)


func SetLogOut(logdir string){

	logf,err := os.OpenFile(filepath.Join(logdir,"logs","logpack.log"),os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil{
		Error("create log file error ", err)
		return
	}
	log.SetOutput(logf)

}


func Info(v ...interface{})  {

	if Level == I || Level == D {
		log.Println(I,v)
	}
}

func Debug(v ...interface{})  {
	if Level == D{
		log.Println(D,v)
	}
}

func Error(v ...interface{})  {
	log.Println(E,v)
}