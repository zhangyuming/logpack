# logpack 
logpack 为服务器日志归档设计的， 服务器上经常有大量的日志文件， 日志文件多且混乱， 该项目旨在整理日志文件

### 同类软件
- logrotate
> logrotate 是日志轮替非常好的一款软件，支持的个性话配置比较强大，但是有个弊端是做日志轮替，轮替完成之后并不能统一归档， 
> 只能自己写脚本配置到配置文件中， logpack 除了可以做日志轮替之外还支持日志归档。


### 典型使用场景
- tomcat 日志归档
> 做过运维的应该比较了解 tomcat logs目录有大量的日志文件， 如果用logrotate 只能轮替 catalina.out， 但是logs文件夹日志文件数量会越来越大.
> 通过logpack 不仅可以轮替catalina.out文件同时还会整理杂乱的日志文件



## 使用介绍 
> - 下载可执行文件[release]到终端服务器 
> - 编写配置文件[/etc/logpack|~/etc/logpack] `配置文件必须为 yaml|yml  配置文件说明见下方` 
> - 启动 `nohup ./logpack-[version] 1>nohup.out 2>&1 &`
> - 默认日志文件为启动目录的 logs/logpack.log
>> 配置文件默认加载目录为 `/etc/logpack` `~/etc/logpack`  
>> eg： /etc/logpack/logpack.yaml   `logpack.yaml`内容参加配置文件示例

## 使用限制
> - 目前只支持linux服务器 
> - 服务器上必须有`lsof`命令

## 参数说明 
```
  -d string
    	指定配置文件路径，默认会加载指定路径所有文件
  -f string
    	指定唯一的配置文件。 如果指定 -f 那么-d将会忽略
  -rate int
    	自定压缩比率 (default 9)
  -t	校验配置文件
  -vv
    	logpack 日志bebug日志级别
```

## TODO
- 动态reload

## 配置文件实例
```
name: "logpack"
archive:
- schedule: "* * 3 * * 6"
  name: "tomcat logs"
  dirs:
    - /usr/local/tomcat/logs
  previous: 1
  rotate: 20

logrotate:
- schedule: "* * 2 * * 6"
  name: "catlina.out"
  files:
    - /usr/local/tomcat/logs/catalina.out
  compress: true
  size: 30M
  rotate: 20
```
配置解读： 每周六凌晨2点会轮替 catalina.out日志（日志文件大小超过30M才会轮替）保留历史20份， 每周六凌晨3点会归档tomcat的logs所有未被程序使用的文件到 logs/logpack 目录
### 配置文件参数说明 
> - name 描述信息无实际意义
> - logrotate 轮替日志
>> - schedule 定时配置见cron配置
>> - name 描述信息 
>> - files 带轮替的日志列表 可以填写多项，为多线程轮替操作 
>> - compress 轮替后的文件是否压缩 
>> - size 日志文件大小限制，当超过辞职日志才会被轮替
>> - rotate 轮替后的文件保留个数
> - archive  归档日志
>> - schedule 定时配置见cron配置
>> - name 描述信息 
>> dirs 归档的文件夹， 默认会把该文件夹下未被其他程序使用的所有文件归档但当前文件夹先的`logpack` 目录 
>> - previous 指定归档几天以前的日志文件  
>> - rotate 保留多少分归档文件 


### cron 配置
引用[robfig/cron](https://godoc.org/github.com/robfig/cron) 详细请查看cron页面
```
Field name   | Mandatory? | Allowed values  | Allowed special characters
----------   | ---------- | --------------  | --------------------------
Seconds      | Yes        | 0-59            | * / , -
Minutes      | Yes        | 0-59            | * / , -
Hours        | Yes        | 0-23            | * / , -
Day of month | Yes        | 1-31            | * / , - ?
Month        | Yes        | 1-12 or JAN-DEC | * / , -
Day of week  | Yes        | 0-6 or SUN-SAT  | * / , - ?
```
