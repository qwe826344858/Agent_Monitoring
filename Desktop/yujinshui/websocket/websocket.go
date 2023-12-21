package main

import (
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
)

const MAGIC_KEY = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

func sayHelloHijacker(w http.ResponseWriter, r *http.Request) {
	fmt.Println("sayHelloHijacker")
	//5. 类型转换，转换为Hijacker接口
	hijacker, _ := w.(http.Hijacker)
	//6. 获取连接和ReadWriter
	conn, buf, _ := hijacker.Hijack()

	buf.WriteString("HTTP/1.1 200 Web Socket Protocol Handshake\r\n")

	//buf.WriteString("Content Length:0\r\n")
	buf.WriteString("\r\n")
	buf.WriteString("hello world")

	buf.Flush()

	//7.关闭连接
	defer conn.Close()
}

func Hello(w http.ResponseWriter, r *http.Request) {

	fmt.Println("listening")
	secWebSocketKey := r.Header.Get("Sec-WebSocket-Key")
	sha1 := sha1.New()
	sha1.Write([]byte(secWebSocketKey + MAGIC_KEY))
	secWsKey := base64.StdEncoding.EncodeToString(sha1.Sum(nil))
	secWsKeyHeader := "Sec-WebSocket-Accept: " + secWsKey + "\r\n"
	fmt.Println(secWsKeyHeader)
	hj := w.(http.Hijacker)
	_, rw, _ := hj.Hijack()

	header := []string{"HTTP/1.1 101 Web Socket Protocol Handshake", "Upgrade: WebSocket", "Connection: Upgrade", secWsKeyHeader}
	s := strings.Join(header, "\r\n")
	fmt.Println(s)
	rw.WriteString(s)
	rw.WriteString("\r\n")
	rw.Flush()
	info := [...]byte{0xE6, 0x88, 0x91, 0xE6, 0x98, 0xAF, 0xE9, 0x98, 0xBF, 0xE5, 0xAE, 0x9D, 0xE5, 0x93, 0xA5}
	maskingKey := [...]byte{0x08, 0xf6, 0xef, 0xb1}
	maskInfo := new([len(info)]byte)

	for i, j := 0, 0; i < len(info); i++ {
		maskInfo[i] = info[i] ^ maskingKey[j]
		j = (i + 1) % 4

	}
	// [FIN, RSV, RSV, RSV, OPCODE, OPCODE, OPCODE, OPCODE];
	var frame1 byte = 0b10000001
	var frame2 byte = 0b00001111

	data := make([]byte, 0)
	data = append(data, frame1, frame2)
	data = append(data, info[:]...)
	rw.Write(data)
	rw.Flush()

	//rw.Write([]byte{0x81,0x85, 0x37, 0xfa, 0x21, 0x3d, 0x7f, 0x9f, 0x4d, 0x51, 0x58})

}

func main() {

	http.HandleFunc("/hello", Hello)
	http.HandleFunc("/hijacker", sayHelloHijacker)

	http.ListenAndServe(":8080", nil)

}
