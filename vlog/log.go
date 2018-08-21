package vlog

import "log"

var Level string = "DEBUG"

const(
	I = "INFO"
	D = "DEBUG"
	E = "ERROR"
)

func Info(v ...interface{})  {

	if Level == I || Level == D {
		log.Print(I,v)
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