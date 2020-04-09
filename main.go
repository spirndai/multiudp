package main

import (
	auth "my.com/goauth"
	"encoding/json"
	"io/ioutil"
	"os"
	"fmt"
	"encoding/base32"
	"time"
	"strconv"
	"math/rand"
	"net"
)
type Conf struct {
	Mode string
	Serverip string
	Remoteport int
	Localport int
	Minport int
	Maxport int
	Secret string
}
func getPorts(seed , size , startport, endport int)[]int {
	n := rand.New(rand.NewSource(int64(seed)))
	tmp := endport - startport
	if (tmp<=size){
		panic("port range too small")
	}
	cache := make(map[int]bool)
	var last int
	for i:=0;i<size;i++{
		last = n.Intn(tmp)
		for {
			if _,ok:=cache[last];ok{
				last = n.Intn(tmp)
			}else{
				break
			}
		}
		cache[last] = true
	}
	var ret  []int
	for i,_:=range cache{
		ret = append(ret,i+startport)
	}
	return ret
}
type ConnInfo struct {
	CreateTime time.Time
	Conns map[int]*net.UDPConn
	Using bool
}
var LastActive *net.UDPConn
var LastAddr *net.UDPAddr
var lasttime int64
var lastconns *ConnInfo
var nowtime int64
var nowconns *ConnInfo
var config *Conf
var localconn *net.UDPConn
var localaddr *net.UDPAddr
var buffersize int 
var configModeServer bool
func initLocal(){
	var err error 
	if (configModeServer){
		t1,_ := net.ResolveUDPAddr("udp",config.Serverip+":"+strconv.Itoa(config.Localport))
		localconn,err = net.DialUDP("udp",nil,t1)
	}else{
		t1,_ := net.ResolveUDPAddr("udp",":"+strconv.Itoa(config.Localport))
		localconn,err = net.ListenUDP("udp",t1)
	}
	if err!=nil{
		panic(err)
	}
	go func(){
		buf:=make([]byte,buffersize)
		var last time.Time
		var conn []int
		var pos int
		for {
			n,addr,err := localconn.ReadFromUDP(buf)
			localaddr = addr
			//fmt.Println("read from local ",n)
			if (err!=nil){
				continue
			}
			if configModeServer {
				//fmt.Println("write to last ")
				if LastActive!=nil{
					LastActive.WriteToUDP(buf[:n],LastAddr)
					//fmt.Println("ok ")
				}
			}else{
				//fmt.Println("write to random ")
				tmpconn := nowconns
				if lastconns!=nil && lastconns.Using{
					tmpconn = lastconns
				}
				if last!=tmpconn.CreateTime{
					last = tmpconn.CreateTime
					conn = nil
					for k,_:=range tmpconn.Conns{
						conn = append(conn,k)
					}
				}
				if pos==0{
					pos = rand.Int()
				}
				c,_ := tmpconn.Conns[conn[pos&0xf]]
				pos >>=  4 
				c.Write(buf[:n])
				//fmt.Println("random ")
			}
		}
	}()
}
func closeConns() {
	if lastconns==nil{
		return 
	}
	for port,item := range lastconns.Conns {
		if  _,ok:=nowconns.Conns[port];!ok {
			item.Close()
		}
	}
}
func newConns(seed int) *ConnInfo{
	ret := getPorts(seed,16,config.Minport,config.Maxport)
	tmp := new(ConnInfo)
	tmp.Conns = make(map[int]*net.UDPConn)
	for _,i:=range ret{
		var t0  *net.UDPConn
		if lastconns!=nil {
			t0,_ = lastconns.Conns[i]
		}
		if t0!=nil{
			tmp.Conns[i] = t0
		}else {
			if (configModeServer){
				t1,_ := net.ResolveUDPAddr("udp",":"+strconv.Itoa(i))
				tmp.Conns[i],_ = net.ListenUDP("udp",t1)
			}else{
				t1,_ := net.ResolveUDPAddr("udp",config.Serverip+":"+strconv.Itoa(i))
				tmp.Conns[i],_ = net.DialUDP("udp",nil,t1)
			}
			go func(c *net.UDPConn){
				buf:=make([]byte,buffersize)
				for {
					if (c==nil){
						fmt.Println("why nil?")
						break
					}
					n,addr,err := c.ReadFromUDP(buf)
					//fmt.Println("read from remote ",addr)
					if err!=nil{
						break
					}
					LastActive = c
					LastAddr = addr
					//fmt.Println("write to local")
					if (configModeServer){
						//fmt.Println("ok2")
						localconn.Write(buf[:n])
					}else if localconn!=nil && localaddr!=nil{
						//fmt.Println("ok1 ",localaddr)
						localconn.WriteToUDP(buf[:n],localaddr)
					}
				}
			}(tmp.Conns[i])
		}
	}
	tmp.CreateTime = time.Now()
	return tmp
}

func makeTimer(){
	c := new(auth.OTPConfig)
	c.Secret = base32.StdEncoding.EncodeToString([]byte(config.Secret))
	timer := time.NewTicker(1*time.Second)
	go func(){
		for _ = range timer.C {
			t0 := time.Now().Unix() 
			t0 = t0 >> 6
			if t0!=lasttime && t0!=nowtime{
				nowtime = t0
				nowconns = newConns(c.ShowCode(t0))
				if lastconns==nil{
					lasttime = nowtime
					lastconns = nowconns
					lastconns.Using = true
					nowtime = 0
					nowconns=nil
				}
			}
			delay := 20
			if nowconns!=nil && nowconns.CreateTime.Add(time.Duration(delay)*time.Second).Before(time.Now()){
				closeConns();
				lasttime = nowtime
				lastconns = nowconns
				nowtime = 0
				nowconns=nil
			}
			if !configModeServer{
				delay = 10
				if nowconns!=nil && nowconns.CreateTime.Add(time.Duration(delay)*time.Second).Before(time.Now()){
					nowconns.Using = true
					if lastconns!=nil{
						lastconns.Using = false
					}
				}
			}

		}


	}()
}
func main() {
	buffersize = 2048
	if len(os.Args)<2{
		fmt.Println(os.Args[0],"conffile")
		return
	}
	fileData,err := ioutil.ReadFile(os.Args[1])
	if err!=nil {
		panic(err)
	}

	err = json.Unmarshal([]byte(fileData),&config)
	if err!=nil{
		panic(err)
	}


	if config.Mode=="server"{
		configModeServer =true
	}
	makeTimer()
	time.Sleep(5*time.Second)
	initLocal()
	fmt.Println("start")
	for {
		time.Sleep(100*time.Second)
	}
}
