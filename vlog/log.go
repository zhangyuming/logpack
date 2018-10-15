package vlog

import (
	"log"
	"io"
	"os"
)

var Level string = "INFO"

var writer io.Writer

const(
	I = "INFO"
	D = "DEBUG"
	E = "ERROR"
)


func SetLogOut(logfile string) (error) {

	logf,err := os.OpenFile(logfile,os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
	log.Print("logging to file : ",logf)
	if err != nil{
		Error("create log file error ", logf)
		return err
	}
	log.SetOutput(logf)

	return nil

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