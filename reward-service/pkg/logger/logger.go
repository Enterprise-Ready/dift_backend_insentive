package logger

import "log"

type Field struct {
	Key   string
	Value any
}
type Logger struct{}

func New(service string) *Logger                    { return (&Logger{}).With(F("service", service)) }
func F(key string, value any) Field                 { return Field{Key: key, Value: value} }
func (l *Logger) With(fields ...Field) *Logger      { return l }
func (l *Logger) Info(msg string, fields ...Field)  { log.Println(msg) }
func (l *Logger) Warn(msg string, fields ...Field)  { log.Println(msg) }
func (l *Logger) Error(msg string, fields ...Field) { log.Println(msg) }
func (l *Logger) Infof(format string, args ...any)  { log.Printf(format, args...) }
func (l *Logger) Warnf(format string, args ...any)  { log.Printf(format, args...) }
func (l *Logger) Errorf(format string, args ...any) { log.Printf(format, args...) }
func (l *Logger) Fatalf(format string, args ...any) { log.Fatalf(format, args...) }
