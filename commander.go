package wsftp

import (
	"fmt"
	"net"
	"strconv"
	"net/http"
	"io/ioutil"
	"github.com/gorilla/websocket"
	"github.com/ecoshub/jint"
	hs "wsftp/hs"
	com "wsftp/tcpcom"
	utils "wsftp/utils"
	cmd "wsftp/cmd"
	router "wsftp/router"
)

const (
	// ports
	STARTPORT int = 9996
	MAINLISTENPORTWS int = 9997
	// 9998 reserved for handshake to handshake com.
	MAINLISTENPORT int = 9999
	// 10000 reserved for handshake to frontend ws com.
	SRLISTENPORT int = 10001
	MSGLISTENPORT int = 10002
	// 10003 reserved for sr to frontend ws com.
	// 10004 reserved for msg to frontend ws com.

	// websocket settings & limits
	ENDPOINT = "/cmd"
	ACTIVETRANSACTIONLIMIT int = 25
)

var (
	activeTransaction int = 0
	ports = make([][]int, ACTIVETRANSACTIONLIMIT)
	portIDMap = make(map[int]string, ACTIVETRANSACTIONLIMIT)
	myIP string = utils.GetInterfaceIP().String()
	commandChan = make(chan []byte, 1)

	upgrader = websocket.Upgrader{
		ReadBufferSize:    1024,
		WriteBufferSize:   1024,
		EnableCompression: false,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
)

func main(){
	initPorts()
	go hs.Start()
	go router.StartRouting()
	go listen()
	go manage()
	http.HandleFunc(ENDPOINT, handleConn)
	err := http.ListenAndServe(":" + strconv.Itoa(MAINLISTENPORTWS), nil)
	fmt.Println("Commander shutdown unexpectedly!", err)
}

func receive(port int) bool {
    strPort := strconv.Itoa(port)
    addr := myIP + ":" + strPort
    tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
    if err != nil {
        fmt.Println("Address resolving error (Inner)",err)
		commandChan <- []byte{0}
        return false
    }
    listener, err := net.ListenTCP("tcp", tcpAddr)
    if err != nil {
        fmt.Println("Listen Error (Inner)", err)
		listener.Close()
		commandChan <- []byte{0}
		return false

    }else{
        defer listener.Close()
    }
    conn, err := listener.Accept()
    if err != nil {
        fmt.Println("Listen Accept Error (Inner) ", err)
		return false
    }
    msg, err :=  ioutil.ReadAll(conn)
    if err != nil {
        fmt.Println("Message Read Error (Inner)", err)
    }else{
	    conn.Close()
		commandChan <- msg
    }
	return true
}

func listen(){
	for {
		receive(MAINLISTENPORT)
	}
}

func handleConn(w http.ResponseWriter, r *http.Request){
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println(err)
	}else{
		fmt.Println("Connection Establish")
		for {
			_, msg, err := ws.ReadMessage()
			if err != nil {
				fmt.Println("Main Command Connection Closed: ", err)
				fmt.Println("Waiting For Another Connection...")
				fmt.Println("If You Are Using 'Commander Tester' Please Restart This Program For synchronization")
				break
			}
			commandChan <- msg
		}
	}
}

func manage(){
	for {
		json := <- commandChan
		event, err := jint.GetString(json, "event")
		if err != nil {parseErrorHandle(err, "event");continue}
		if event != ""{
			switch event{
			case "actv":
				cmd.TransmitData(myIP, SRLISTENPORT, fmt.Sprintf(`{"event":"actv","total":"%v","active":"%v"}`, ACTIVETRANSACTIONLIMIT, activeTransaction))
			case "my":
				cmd.TransmitData(myIP, SRLISTENPORT,fmt.Sprintf(`{"event":"my","username":"%v","mac":"%v","ip":"%v","nick":"%v"}`, utils.GetUsername(), utils.GetEthMac(), myIP, utils.GetNick()))
			case "creq":
				dir, err := jint.GetString(json, "dir")
				if err != nil {parseErrorHandle(err, "dir");continue}
				if activeTransaction < ACTIVETRANSACTIONLIMIT {
					mac, err := jint.GetString(json, "mac")
					if err != nil {parseErrorHandle(err, "mac");continue}
					uuid, err := jint.GetString(json, "uuid")
					if err != nil {parseErrorHandle(err, "uuid");continue}
					ip, err := jint.GetString(json, "ip")
					if err != nil {parseErrorHandle(err, "ip");continue}
					username, err := jint.GetString(json, "username")
					if err != nil {parseErrorHandle(err, "username");continue}
					nick, err := jint.GetString(json, "nick")
					if err != nil {parseErrorHandle(err, "nick");continue}
					if utils.IsDir(dir) {
						cmd.TransmitData(myIP, SRLISTENPORT,`{"event":"info","content":"Folder transaction not suppoted."}`)
						continue
					}else{
						cmd.SendRequest(ip, dir, mac, username, nick, uuid)
						activeTransaction++
						continue
					}
				}else{
					cmd.TransmitData(myIP, SRLISTENPORT,fmt.Sprintf(`{"event":"info","content":"Active transaction full","dir":"%v"}`, dir))
					continue
				}
			case "cacp":
				dir, err := jint.GetString(json, "dir")
				if err != nil {parseErrorHandle(err, "dir");continue}
				if activeTransaction < ACTIVETRANSACTIONLIMIT {
					dest, err := jint.GetString(json, "dest")
					if err != nil {parseErrorHandle(err, "dest");continue}
					uuid, err := jint.GetString(json, "uuid")
					if err != nil {parseErrorHandle(err, "uuid");continue}
					mac, err := jint.GetString(json, "mac")
					if err != nil {parseErrorHandle(err, "mac");continue}
					ip, err := jint.GetString(json, "ip")
					if err != nil {parseErrorHandle(err, "ip");continue}
					username, err := jint.GetString(json, "username")
					if err != nil {parseErrorHandle(err, "username");continue}
					nick, err := jint.GetString(json, "nick")
					if err != nil {parseErrorHandle(err, "nick");continue}
					index := allocatePort()
					if index == -1 {
						cmd.TransmitData(myIP,SRLISTENPORT,fmt.Sprintf(`{"event":"info","content":"Active transaction full"}`))
						cmd.SendReject(ip, mac, dir, uuid, username, nick, "full")
						continue
					}
					newPort := ports[index][0]
					portIDMap[newPort] = uuid
					go com.ReceiveFile(ip, mac, username, nick, newPort, uuid, &(ports[index][1]))
					cmd.SendAccept(ip, mac, dir, dest, username, nick, uuid, newPort)
					activeTransaction++
				}else{
					cmd.TransmitData(myIP, SRLISTENPORT,fmt.Sprintf(`{"event":"info","content":"Active transaction full","dir":"%v"}`, dir))
				}
			case "crej":
				mac, err := jint.GetString(json, "mac")
				if err != nil {parseErrorHandle(err, "mac");continue}
				dir, err := jint.GetString(json, "dir")
				if err != nil {parseErrorHandle(err, "dir");continue}
				uuid, err := jint.GetString(json, "uuid")
				if err != nil {parseErrorHandle(err, "uuid");continue}
				ip, err := jint.GetString(json, "ip")
				if err != nil {parseErrorHandle(err, "ip");continue}
				username, err := jint.GetString(json, "username")
				if err != nil {parseErrorHandle(err, "username");continue}
				nick, err := jint.GetString(json, "nick")
				if err != nil {parseErrorHandle(err, "nick");continue}
				cmd.SendReject(ip, mac, dir, uuid, username, nick, "standart")
			case "cmsg":
				mac, err := jint.GetString(json, "mac")
				if err != nil {parseErrorHandle(err, "mac");continue}
				msg, err := jint.GetString(json, "msg")
				if err != nil {parseErrorHandle(err, "msg");continue}
				ip, err := jint.GetString(json, "ip")
				if err != nil {parseErrorHandle(err, "ip");continue}
				username, err := jint.GetString(json, "username")
				if err != nil {parseErrorHandle(err, "username");continue}
				nick, err := jint.GetString(json, "nick")
				if err != nil {parseErrorHandle(err, "nick");continue}
				cmd.SendMessage(ip, mac, username, nick, msg)
			case "racp":
				dir, err := jint.GetString(json, "dir")
				if err != nil {parseErrorHandle(err, "dir");continue}
				dest, err := jint.GetString(json, "dest")
				if err != nil {parseErrorHandle(err, "dest");continue}
				uuid, err := jint.GetString(json, "uuid")
				if err != nil {parseErrorHandle(err, "uuid");continue}
				mac, err := jint.GetString(json, "mac")
				if err != nil {parseErrorHandle(err, "mac");continue}
				ip, err := jint.GetString(json, "ip")
				if err != nil {parseErrorHandle(err, "ip");continue}
				port, err := jint.GetString(json, "port")
				if err != nil {parseErrorHandle(err, "port");continue}
				username, err := jint.GetString(json, "username")
				if err != nil {parseErrorHandle(err, "username");continue}
				nick, err := jint.GetString(json, "nick")
				if err != nil {parseErrorHandle(err, "nick");continue}
				intPort, _ := strconv.Atoi(port)
				index := getPortIndex(intPort)
				if !setPortBusy(intPort) {
					// add an error handle if needed.
					continue
				}
				go com.SendFile(ip, mac, username, nick, intPort, uuid, dir, dest, &(ports[index][1]))
			case "cncl":
				dir, err := jint.GetString(json, "dir")
				if err != nil {parseErrorHandle(err, "dir");continue}
				uuid, err := jint.GetString(json, "uuid")
				if err != nil {parseErrorHandle(err, "uuid");continue}
				mac, err := jint.GetString(json, "mac")
				if err != nil {parseErrorHandle(err, "mac");continue}
				ip, err := jint.GetString(json, "ip")
				if err != nil {parseErrorHandle(err, "ip");continue}
				username, err := jint.GetString(json, "username")
				if err != nil {parseErrorHandle(err, "username");continue}
				nick, err := jint.GetString(json, "nick")
				if err != nil {parseErrorHandle(err, "nick");continue}
				cmd.SendCancel(ip, dir, mac, username, nick, uuid)
			case "dprg":
				port, err := jint.GetString(json, "port")
				if err != nil {parseErrorHandle(err, "port");continue}
				intPort, _ := strconv.Atoi(port)
				if !freePort(intPort){
					// add an error handle if needed.
					continue
				}
				activeTransaction--

			case "fprg":
				port, err := jint.GetString(json, "port")
				if err != nil {parseErrorHandle(err, "port");continue}
				intPort, _ := strconv.Atoi(port)
				freePort(intPort)
				activeTransaction--
			case "kprg":
				port, err := jint.GetString(json, "port")
				if err != nil {parseErrorHandle(err, "port");continue}
				intPort, _ := strconv.Atoi(port)
				freePort(intPort)
			case "rshs":
				hs.Restart()
			default:
				cmd.TransmitData(myIP,SRLISTENPORT,`{"event":"info","content":"Wrong command"}`)
			}
		}else{
			cmd.TransmitData(myIP,SRLISTENPORT,`{"event":"info","content":"'event' can't be null"}`)
		}
	}
}

func parseErrorHandle(err error, key string){
	cmd.TransmitData(myIP, SRLISTENPORT, fmt.Sprintf(`{"event":"error","from":"commander/manage","content":"%v","key":"%v"}`, err, key))
}

func allocatePort() int{
	for i := 0 ; i < ACTIVETRANSACTIONLIMIT ; i++ {
		if ports[i][1] == 0  && portCheck(ports[i][0]){
			ports[i][1] = 1
			return i
		}
	}
	return -1
}

func portCheck(port int) bool{
    strPort := strconv.Itoa(port)
    listener, err := net.Listen("tcp", ":" + strPort)
    if err != nil {
       return false
    }
    listener.Close()
    return true
}

func initPorts(){
	for i := 0 ; i < ACTIVETRANSACTIONLIMIT ; i++ {
		if portCheck(STARTPORT - i){
			ports[i] = []int{STARTPORT - i, 0}
		}
	}
}

func getPortIndex(port int) int{
	for i := 0 ; i < ACTIVETRANSACTIONLIMIT ; i++ {
		if ports[i][0] == port {
			return i
		}
	}
	fmt.Println("Fatal error (getPort) port index out of range", port)
	return -1
}

func setPortBusy(port int) bool{
	index := getPortIndex(port)
	if index > -1 && index < ACTIVETRANSACTIONLIMIT {
		ports[index][1] = 1
		return true
	}else{
		fmt.Println("Fatal error (setPort) port index out of range", port, index)
		return false
	}
}

func freePort(port int) bool{
	index := getPortIndex(port)
	if index > -1 && index < ACTIVETRANSACTIONLIMIT {
		ports[index][1] = 0
		return true
	}else{
		fmt.Println("Fatal error (freeport) port index out of range", port, index)
		return false
	}
}