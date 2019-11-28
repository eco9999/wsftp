package main

import (
	"fmt"
	"net"
	"strconv"
	"net/http"
	"io/ioutil"
	"github.com/gorilla/websocket"
	hs "wsftp/hs"
	com "wsftp/tcpcom"
	utils "wsftp/utils"
	cmd "wsftp/cmd"
	router "wsftp/router"
	json "wsftp/json"
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
	ACTIVEDOWNLOADLIMIT int = 25
	ACTIVEUPLOADLIMIT int = 25
)

var (
	activeUpload int = 0
	activeDownload int = 0
	ports = make([][]int, ACTIVEDOWNLOADLIMIT)
	portIDMap = make(map[int]string, ACTIVEDOWNLOADLIMIT)
	myIP string = utils.GetInterfaceIP().String()
	myUserName string = utils.GetUsername()
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
		receivedCommand := string(<- commandChan)
		commandJSON := json.JTM(receivedCommand)
		result, event := getVal(commandJSON, "event")
		if !result {continue}
		if event != ""{
			switch event{
			case "creq":
				if activeUpload < ACTIVEUPLOADLIMIT {
					result, dir := getVal(commandJSON, "dir")
					if !result {continue}
					result, mac := getVal(commandJSON, "mac")
					if !result {continue}
					ip := hs.GetIP(mac)
					username := hs.GetUsername(mac)
					cmd.SendRequest(ip, dir, mac, username)
					activeUpload++
				}else{
					cmd.TransmitData(myIP,SRLISTENPORT,`{"event":"info","content":"activeUploadFull"}`)
				}
			case "cacp":
				index := allocatePort()
				newPort := ports[index][0]
				result, dir := getVal(commandJSON, "dir")
				if !result {continue}
				result, dest := getVal(commandJSON, "dest")
				if !result {continue}
				result, id := getVal(commandJSON, "id")
				if !result {continue}
				result, mac := getVal(commandJSON, "mac")
				if !result {continue}
				username := hs.GetUsername(mac)
				ip := hs.GetIP(mac)
				if newPort == -1{
					cmd.TransmitData(myIP,SRLISTENPORT,`{"event":"info","content":"activeDownloadFull"}`)
					cmd.SendReject(ip, mac, dir, username)
				}else{
					portIDMap[newPort] = id
					go com.ReceiveFile(ip, mac, username, newPort, id, &(ports[index][1]))
					cmd.SendAccept(ip, mac, dir, dest, username, id, newPort)
				}
			case "crej":
				result, mac := getVal(commandJSON, "mac")
				if !result {continue}
				ip := hs.GetIP(mac)
				username := hs.GetUsername(mac)
				cmd.SendReject(ip, mac, commandJSON["dir"][0], username)
			case "cmsg":
				result, mac := getVal(commandJSON, "mac")
				if !result {continue}
				result, msg := getVal(commandJSON, "msg")
				if !result {continue}
				ip := hs.GetIP(mac)
				username := hs.GetUsername(mac)
				cmd.SendMessage(ip, mac, username, msg)
			case "racp":
				result, dir := getVal(commandJSON, "dir")
				if !result {continue}
				result, dest := getVal(commandJSON, "destination")
				if !result {continue}
				result, id := getVal(commandJSON, "id")
				if !result {continue}
				result, mac := getVal(commandJSON, "mac")
				if !result {continue}
				result, ip := getVal(commandJSON, "ip")
				if !result {continue}
				result, port := getVal(commandJSON, "port")
				if !result {continue}
				intPort, _ := strconv.Atoi(port)
				index := getPortIndex(intPort)
				username := hs.GetUsername(mac)
				setPortBusy(intPort)
				go com.SendFile(ip, mac, username, intPort, id, dir, dest, &(ports[index][1]))
			case "dprg":
				result, port := getVal(commandJSON, "port")
				if !result {continue}
				intPort, _ := strconv.Atoi(port)
				freePort(intPort)
			case "fprg":
				result, port := getVal(commandJSON, "port")
				if !result {continue}
				intPort, _ := strconv.Atoi(port)
				freePort(intPort)
			case "kprg":
				result, port := getVal(commandJSON, "port")
				if !result {continue}
				intPort, _ := strconv.Atoi(port)
				freePort(intPort)
			case "reshs":
				hs.Restart()
			}
		}else{
			fmt.Println("Wrong command")
		}
	}
}

func allocatePort() int{
	for i := 0 ; i < ACTIVEDOWNLOADLIMIT ; i++ {
		if ports[i][1] == 0  && portCheck(ports[i][0]){
			ports[i][1] = 1
			return i
		}
	}
	return -1
}

func getVal(json map[string][]string, key string) (bool, string){
	if len(json[key]) == 0 {
		fmt.Printf("Missing key '%v' from map '%v'", key, json)
	}else{
		return true, json[key][0]
	}
	return false, "null"
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
	for i := 0 ; i < ACTIVEDOWNLOADLIMIT ; i++ {
		if portCheck(STARTPORT - i){
			ports[i] = []int{STARTPORT - i, 0}
		}
	}
}

func getPortIndex(port int) int{
	for i := 0 ; i < ACTIVEDOWNLOADLIMIT ; i++ {
		if ports[i][0] == port {
			return i
		}
	}
	fmt.Println("Fatal error port index out of range", port)
	return -1
}

func setPortBusy(port int) {
	index := getPortIndex(port)
	ports[index][1] = 1
}

func freePort(port int){
	index := getPortIndex(port)
	ports[index][1] = 0 
}