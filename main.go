package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/L7-MCPE/lav7"
	"github.com/L7-MCPE/lav7/raknet"
	"github.com/L7-MCPE/lav7/util/buffer"
	"github.com/davecgh/go-spew/spew"
)

var conn net.Conn
var lastSeq uint32
var ocr1, ocr2, handshake []byte
var target *net.UDPAddr

func main() {
	if len(os.Args) < 2 {
		fmt.Printf("Usage: %s <target address:port>", os.Args[0])
		os.Exit(0)
	}
	var err error
	conn, err = net.Dial("udp", os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	target, _ = net.ResolveUDPAddr("udp", os.Args[1])
	boilup()
	auth()
	<-make(chan bool)
}

func boilup() {
	raknet.InitProtocol()
	ocr1 = raknet.GetHandler(0x05).Write(raknet.Fields{
		"protocol": uint8(raknet.RaknetProtocol),
		"mtuSize":  1464,
	}).Done()
	ocr2 = raknet.GetHandler(0x07).Write(raknet.Fields{
		"serverAddress": func() *net.UDPAddr {
			n, _ := net.ResolveUDPAddr("udp", os.Args[1])
			return n
		}(),
		"mtuSize":  uint16(1464),
		"clientID": uint64(483287947),
	}).Done()
	dp := &raknet.DataPacket{
		Head:      0x80,
		SeqNumber: 0,
		Packets: []*raknet.EncapsulatedPacket{
			&raknet.EncapsulatedPacket{
				Buffer: raknet.GetDataHandler(0x13).Write(raknet.Fields{
					"address": new(net.UDPAddr),
					"systemAddresses": func() []*net.UDPAddr {
						r := make([]*net.UDPAddr, 10)
						for i := 0; i < 10; i++ {
							r[i] = new(net.UDPAddr)
						}
						return r
					}(),
					"sendPing": uint64(324324),
					"sendPong": uint64(324325),
				}),
			},
		},
	}
	dp.Encode()
	handshake = dp.Done()
}

func auth() {
	go func() {
		for {
			recvBuffer := make([]byte, 1024*4)
			if _, err := bufio.NewReader(conn).Read(recvBuffer); err != nil {
				panic(err)
			}
			spew.Dump(recvBuffer)
			if len(recvBuffer) >= 8 &&
				recvBuffer[0] >= 0x80 && recvBuffer[0] < 0x90 &&
				recvBuffer[4] == 0x00 && recvBuffer[7] == 0x15 {
				fmt.Println("Session is closed!")
				os.Exit(0)
			} else if recvBuffer[0] >= 0x80 && recvBuffer[0] < 0x90 {
				dp := &raknet.DataPacket{
					Buffer: buffer.FromBytes(recvBuffer),
				}
				dp.Decode()
				for _, ep := range dp.Packets {
					if ep.Buffer.Payload[0] == 0x91 {
						fmt.Println("Session is closed!")
						os.Exit(0)
						return
					}
				}
			}
		}
	}()

	send(ocr1)
	send(ocr2)
	send(handshake)
	lastSeq++
	sendCompressed(&lav7.Login{
		Username: "Helloworld",
		Proto1:   lav7.MinecraftProtocol,
		Proto2:   lav7.MinecraftProtocol,
		ClientID: 0,
		SkinName: "WhoKnows",
		Skin:     make([]byte, 32*64*4),
	})
	lastSeq++
}

func sendFromFields(pid byte, field raknet.Fields) {
	send(append([]byte{pid}, raknet.GetHandler(pid).Write(field).Done()[1:]...))
	return
}

func sendDataPacket(pid byte, field raknet.Fields) {
	dp := &raknet.DataPacket{
		Head:      0x80,
		SeqNumber: lastSeq,
		Packets: []*raknet.EncapsulatedPacket{
			&raknet.EncapsulatedPacket{
				Buffer: raknet.GetDataHandler(pid).Write(field),
			},
		},
	}
	dp.Encode()
	send(dp.Done())
	lastSeq++
}

func sendPacket(pk lav7.Packet) {
	sendFromFields(0x80, raknet.Fields{
		"seqNumber": lastSeq,
		"packets": []*raknet.EncapsulatedPacket{
			raknet.NewEncapsulated(
				func() *buffer.Buffer {
					buf := buffer.FromBytes([]byte{pk.Pid()})
					buf.Append(pk.Write())
					return buf
				}(),
			),
		},
	})
	lastSeq++
}

func sendCompressed(pk lav7.Packet) {
	b := append([]byte{pk.Pid()}, pk.Write().Done()...)
	sendPacket(&lav7.Batch{
		Payloads: [][]byte{b},
	})
}

func send(b []byte) {
	conn.Write(b)
}
