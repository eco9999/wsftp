package commands

import (
	"fmt"
	"net"
	"strconv"
	utils "wsftp/utils"
    rw "github.com/ecoshub/penman"  
)

const (
    // 9996 reserverd for transfer start port
    // 9997 reserverd ws commander comminication.
    // 9998 reserverd tcp handshake comminication.
    MAINLISTEN int = 9999
    // 10000 reserved for handshake
    SRLISTEN int = 10001
    MSGLISTEN int = 10002
    // 10003 reserved for ws sr
    // 10004 reserved for ws msg
)

var (
    myIP string = utils.GetInterfaceIP().String()
    myUsername string = utils.GetUsername()
    myNick string = utils.GetNick()
    myMAC string = utils.GetEthMac()
)

func SendRequest(ip, dir, mac, username, nick, uuid string){
    dir = rw.PreProcess(dir)
    fileSize := utils.GetFileSize(dir)
    fileName := utils.GetFileName(dir)
    fileType := utils.GetFileExt(fileName)

    if fileSize == 0 {
        fnf := `{"event":"info","content":"File not found"}`
        TransmitData(myIP, SRLISTEN, fnf)
    }else{
        dataToSend := fmt.Sprintf(`"username":"%v","nick":"%v","ip":"%v","mac":"%v","dir":"%v","fileName":"%v","fileType":"%v","fileSize":"%v","contentType":"file","uuid":"%v"}`,
         myUsername, myNick, myIP, myMAC, dir, fileName, fileType, strconv.FormatInt(fileSize, 10), uuid)
        dataToMe := fmt.Sprintf(`"username":"%v","nick":"%v","ip":"%v","mac":"%v","dir":"%v","fileName":"%v","fileType":"%v","fileSize":"%v","contentType":"file","uuid":"%v"}`,
         username, nick, ip, mac, dir, fileName, fileType, strconv.FormatInt(fileSize, 10), uuid)

        rreq := `{"event":"rreq",` + dataToSend
        sreq := `{"event":"sreq",` + dataToMe
        freq := `{"event":"freq",` + dataToMe
        
        res := TransmitData(ip, SRLISTEN, rreq)
        if res {
            TransmitData(myIP, SRLISTEN, sreq)
        }else{
            TransmitData(myIP, SRLISTEN, freq)
        }
    }
}

func SendCancel(ip, dir, mac, username, nick, uuid string){
    dir = rw.PreProcess(dir)
    fileSize := utils.GetFileSize(dir)
    fileName := utils.GetFileName(dir)
    fileType := utils.GetFileExt(fileName)

    dataToSend := fmt.Sprintf(`"username":"%v","nick":"%v","ip":"%v","mac":"%v","dir":"%v","fileName":"%v","fileType":"%v","fileSize":"%v","contentType":"file","uuid":"%v"}`,
     myUsername, myNick, myIP, myMAC, dir, fileName, fileType, strconv.FormatInt(fileSize, 10), uuid)
    dataToMe := fmt.Sprintf(`"username":"%v","nick":"%v","ip":"%v","mac":"%v","dir":"%v","fileName":"%v","fileType":"%v","fileSize":"%v","contentType":"file","uuid":"%v"}`,
     username, nick, ip, mac, dir, fileName, fileType, strconv.FormatInt(fileSize, 10), uuid)

    rreq := `{"event":"rcncl",` + dataToSend
    sreq := `{"event":"scncl",` + dataToMe
    freq := `{"event":"fcncl",` + dataToMe
    
    res := TransmitData(ip, SRLISTEN, rreq)
    if res {
        TransmitData(myIP, SRLISTEN, sreq)
    }else{
        TransmitData(myIP, SRLISTEN, freq)
    }
}

func SendAccept(ip, mac, dir, dest, username, nick, uuid string, port int){
    fileName := utils.GetFileName(dir)
    fileType := utils.GetFileExt(fileName)

    dataToSend := fmt.Sprintf(`"username":"%v","nick":"%v","ip":"%v","mac":"%v","dir":"%v","fileName":"%v","fileType":"%v","destination":"%v","port":"%v","uuid":"%v","contentType":"file"}`,
        myUsername, myNick, myIP, myMAC, dir, fileName, fileType, dest, strconv.Itoa(port), uuid)
    dataToMe := fmt.Sprintf(`"username":"%v","nick":"%v","ip":"%v","mac":"%v","dir":"%v","fileName":"%v","fileType":"%v","destination":"%v","port":"%v","uuid":"%v","contentType":"file"}`,
        username, nick, ip, mac, dir, fileName, fileType, dest, strconv.Itoa(port), uuid)

    racp := `{"event":"racp",` + dataToSend
    sacp := `{"event":"sacp",` + dataToMe
    facp := `{"event":"facp",` + dataToMe
    
    res := TransmitData(ip, MAINLISTEN, racp)
    if res {
        TransmitData(myIP, SRLISTEN, sacp)
        TransmitData(ip, SRLISTEN, racp)
    }else{
        TransmitData(ip, SRLISTEN, facp)
        TransmitData(myIP, SRLISTEN, facp)
    }
}

func SendReject(ip, mac, dir, uuid, username, nick, cause string){
    fileName := utils.GetFileName(dir)
    fileType := utils.GetFileExt(fileName)

    dataToSend := fmt.Sprintf(`"username":"%v","nick":"%v","ip":"%v","mac":"%v","dir":"%v","fileName":"%v","fileType":"%v","contentType":"file","uuid":"%v","cause":"%v"}`,
     myUsername, myNick, myIP, myMAC, dir, fileName, fileType, uuid, cause)
    dataToMe := fmt.Sprintf(`"username":"%v","nick":"%v","ip":"%v","mac":"%v","dir":"%v","fileName":"%v","fileType":"%v","contentType":"file","uuid":"%v","cause":"%v"}`,
     username, nick, ip, mac, dir, fileName, fileType, uuid, cause)

    rrej := `{"event":"rrej",` + dataToSend
    srej := `{"event":"srej",` + dataToMe
    frej := `{"event":"frej",` + dataToMe

    res := TransmitData(ip, SRLISTEN, rrej)
    if res {
    	TransmitData(myIP, SRLISTEN, srej)
    }else{
    	TransmitData(myIP, SRLISTEN, frej)
    }
}

func SendMessage(ip, mac, username, nick, msg string){
    dataToSend := fmt.Sprintf(`"mac":"%v","username":"%v","nick":"%v","content":"%v","contentType":"text"}`, myMAC, myUsername, myNick, msg)
    dataToMe := fmt.Sprintf(`"mac":"%v","username":"%v","nick":"%v","content":"%v","contentType":"text"}`,mac,  username, nick, msg)

    rmsg := `{"event":"rmsg",` + dataToSend
    smsg := `{"event":"smsg",` + dataToMe
    fmsg := `{"event":"fmsg",` + dataToMe

    res := TransmitData(ip,MSGLISTEN, rmsg)
    if res {
        TransmitData(myIP, MSGLISTEN, smsg)
    }else{
        TransmitData(myIP, MSGLISTEN, fmsg)
    }
}

func TransmitData(ip string, port int, msg string) bool{
    strPort := strconv.Itoa(port)
    addr := ip + ":" + strPort
    tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
    if err != nil {
        fmt.Println("Address resolving error (Inner)", err)
        return false
    }
    conn, err := net.DialTCP("tcp", nil, tcpAddr)
    if err != nil {
        fmt.Println("Connection Fail (Inner)", err)
        return false
    }else{
        conn.Write([]byte(msg))
        conn.Close()
        return true
    }
}