package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"math/rand"
	"time"
)

func ntlm2_calc_resp(passnt2, challenge string, tbofs, tblen int) (nthash []byte, ntlen int, lmhash []byte, lmlen int) {

	nthash = []byte{}
	ntlen = 0

	lmhash = []byte{}
	lmlen = 0

	nonce := bytes.NewBuffer(make([]byte, 0, 8+1))
	rand.Seed(time.Now().UTC().UnixNano())
	binary.Write(nonce, binary.BigEndian, uint64(rand.Uint32())<<32+uint64(rand.Uint32()))

	var tw uint64
	tw = (uint64(time.Now().Unix()) + uint64(11644473600)) * uint64(10000000)

	blob := bytes.NewBuffer(make([]byte, 0, 4+4+8+8+4+tblen+4+1))
	binary.Write(blob, binary.LittleEndian, int32(0x00000101))
	binary.Write(blob, binary.LittleEndian, int32(0))
	binary.Write(blob, binary.LittleEndian, tw)
	binary.Write(blob, binary.LittleEndian, nonce.Bytes()[0:7])
	binary.Write(blob, binary.LittleEndian, int32(0))
	binary.Write(blob, binary.LittleEndian, challenge[tbofs:tblen-1])
	binary.Write(blob, binary.LittleEndian, int32(0))
	fmt.Printf("blob: ")
	for i := 0; i < len(blob.Bytes()); i++ {
		fmt.Printf("%d ", blob.Bytes()[i])
	}
	fmt.Printf(" (%d -> %d)\n", len(blob.Bytes()), 4+4+8+8+4+tblen+4+1)

	blen := 28 + tblen + 4

	ntlen = 16 + blen

	nthashBuffer := bytes.NewBuffer(make([]byte, 0, ntlen+1))
	buf := bytes.NewBuffer(make([]byte, 0, 8+blen+1))

	binary.Write(buf, binary.BigEndian, challenge[23:31])
	binary.Write(buf, binary.BigEndian, blob.Bytes()[:blen-1])

	hmacmd5 := hmac.New(md5.New, []byte(passnt2))
	binary.Write(nthashBuffer, binary.BigEndian, hmacmd5.Sum(buf.Bytes()[:8+blen-1]))

	nthash = nthashBuffer.Bytes()

	lmlen = 24
	lmhashBuffer := bytes.NewBuffer(make([]byte, 0, lmlen+1))
	buf = bytes.NewBuffer(make([]byte, 0, 16+1))
	binary.Write(buf, binary.BigEndian, challenge[23:31])
	binary.Write(buf, binary.BigEndian, nonce.Bytes()[0:7])

	hmacmd5 = hmac.New(md5.New, []byte(passnt2))
	binary.Write(lmhashBuffer, binary.BigEndian, hmacmd5.Sum(buf.Bytes()[:15]))

	fmt.Printf("nonce: %v\ntw: %v\nblob: %v\nnthash: %v\nlmhash: %v", nonce.Bytes(), tw, blob.Bytes(), nthash, lmhash)

	return
}

func main() {
	fmt.Println("ntlm")
	passnt2 := "passnt2"
	challenge := "a very long challengeABCDEFGHIJKLMNOPQRSTUVWXYZ"
	ntlm2_calc_resp(passnt2, challenge, 0, len(challenge))
}
