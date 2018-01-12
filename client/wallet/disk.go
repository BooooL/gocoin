package wallet

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"fmt"
	"github.com/piotrnar/gocoin/client/common"
	"github.com/piotrnar/gocoin/lib/btc"
	"github.com/piotrnar/gocoin/lib/utxo"
	"io/ioutil"
	"os"
	"reflect"
	"time"
	"unsafe"
)

const (
	FILE_NAME = "balances.db"
)

var (
	END_MARKER     = []byte("END_OF_FILE")
	file_for_block [32]byte
)

/*
   Format of balances.db file:

     block hash - 32 bytes
     common.AllBalMinVal - var_int
     utxo.UtxoIdxLen - 1 byte

     number of P2KH records - var_int
     each record: hash[20] || value[var_int] || count[var_int] || inp[utxo.UtxoIdxLen + 4]

     number of P2SH records - var_int
     each record: hash[20] || value[var_int] || count[var_int] || inp[utxo.UtxoIdxLen + 4]

     marker: "END_OF_FILE" -- 11 bytes
*/

// return false if failed
func Load(abort *bool) bool {
	var ha [32]byte
	var ui uint64
	f, er := os.Open(common.GocoinHomeDir + FILE_NAME)
	if er != nil {
		println(er.Error())
		return false
	}
	defer f.Close()

	fmt.Println("Loading balances of", btc.UintToBtc(common.AllBalMinVal()), "BTC or more from", FILE_NAME)

	rd := bufio.NewReader(f)
	er = btc.ReadAll(rd, ha[:])
	if er != nil {
		println(er.Error())
		return false
	}
	if !bytes.Equal(ha[:], common.Last.Block.BlockHash.Hash[:]) {
		println(FILE_NAME, "is for different last block hash")
		return false
	}

	ui, er = btc.ReadVLen(rd)
	if er != nil {
		println(er.Error())
		return false
	}
	if ui != common.AllBalMinVal() {
		println(FILE_NAME, "is for different AllBalMinVal")
		return false
	}

	er = btc.ReadAll(rd, ha[:1])
	if er != nil {
		println(er.Error())
		return false
	}
	if ha[0] != byte(utxo.UtxoIdxLen) {
		println(FILE_NAME, "is for different UtxoIdxLen")
		return false
	}

	AllBalancesP2KH, er = load_one_map(rd, "P2KH", abort)
	if er != nil {
		println("AllBalancesP2KH:", er.Error())
		return false
	}
	if *abort {
		return false
	}

	AllBalancesP2SH, er = load_one_map(rd, "P2SH", abort)
	if er != nil {
		println("AllBalancesP2SH:", er.Error())
		return false
	}
	if *abort {
		return false
	}

	AllBalancesP2WKH, er = load_one_map(rd, "P2WKH", abort)
	if er != nil {
		println("AllBalancesP2WKH:", er.Error())
		return false
	}
	if *abort {
		return false
	}

	AllBalancesP2WSH, er = load_one_map32(rd, "P2WSH", abort)
	if er != nil {
		println("AllBalancesP2WSH:", er.Error())
		return false
	}
	if *abort {
		return false
	}

	er = btc.ReadAll(rd, ha[:len(END_MARKER)])
	if er != nil {
		println("END_MARKER read:", er.Error())
		return false
	}
	if !bytes.Equal(ha[:len(END_MARKER)], END_MARKER) {
		println(FILE_NAME, "has marker missing")
		return false
	}

	copy(file_for_block[:], common.Last.Block.BlockHash.Hash[:])
	return true
}

func load_one_map(rd *bufio.Reader, what string, abort *bool) (res map[[20]byte]*OneAllAddrBal, er error) {
	var recs, outs, cnt_dwn_from, cnt_dwn uint64
	var key [20]byte
	var bts, perc int
	var slice []byte
	var v *OneAllAddrBal

	recs, er = btc.ReadVLen(rd)
	if er != nil {
		return
	}

	what = fmt.Sprint(recs, " ", what, " addresses")
	cnt_dwn_from = recs / 100

	allbal := make(map[[20]byte]*OneAllAddrBal, int(recs))

	for ; recs > 0; recs-- {
		er = btc.ReadAll(rd, key[:])
		if er != nil {
			return
		}
		v = new(OneAllAddrBal)
		v.Value, er = btc.ReadVLen(rd)
		if er != nil {
			return
		}

		outs, er = btc.ReadVLen(rd)
		if er != nil {
			return
		}

		if int(outs) >= common.CFG.AllBalances.UseMapCnt-1 {
			// using map
			var tmp OneAllAddrInp
			v.unspMap = make(map[OneAllAddrInp]bool, int(outs))
			for ; outs > 0; outs-- {
				er = btc.ReadAll(rd, tmp[:])
				if er != nil {
					return
				}
				v.unspMap[tmp] = true
			}
		} else {
			// using list
			v.unsp = make([]OneAllAddrInp, int(outs))
			bts = len(v.unsp) * len(v.unsp[0])
			slice = *(*[]byte)(unsafe.Pointer(&reflect.SliceHeader{Data: uintptr(unsafe.Pointer(&v.unsp[0][0])), Len: bts, Cap: bts}))
			er = btc.ReadAll(rd, slice)
			if er != nil {
				return
			}
		}

		if *abort {
			return
		}

		allbal[key] = v

		if cnt_dwn == 0 {
			fmt.Print("\rLoading ", what, " - ", perc, "% complete ... ")
			perc++
			cnt_dwn = cnt_dwn_from
		} else {
			cnt_dwn--
		}
	}

	fmt.Print("\r                                                              \r")

	// all good
	res = allbal
	return
}

func load_one_map32(rd *bufio.Reader, what string, abort *bool) (res map[[32]byte]*OneAllAddrBal, er error) {
	var recs, outs, cnt_dwn_from, cnt_dwn uint64
	var key [32]byte
	var bts, perc int
	var slice []byte
	var v *OneAllAddrBal

	recs, er = btc.ReadVLen(rd)
	if er != nil {
		return
	}

	what = fmt.Sprint(recs, " ", what, " addresses")
	cnt_dwn_from = recs / 100

	allbal := make(map[[32]byte]*OneAllAddrBal, int(recs))

	for ; recs > 0; recs-- {
		er = btc.ReadAll(rd, key[:])
		if er != nil {
			return
		}
		v = new(OneAllAddrBal)
		v.Value, er = btc.ReadVLen(rd)
		if er != nil {
			return
		}

		outs, er = btc.ReadVLen(rd)
		if er != nil {
			return
		}

		if int(outs) >= common.CFG.AllBalances.UseMapCnt-1 {
			// using map
			var tmp OneAllAddrInp
			v.unspMap = make(map[OneAllAddrInp]bool, int(outs))
			for ; outs > 0; outs-- {
				er = btc.ReadAll(rd, tmp[:])
				if er != nil {
					return
				}
				v.unspMap[tmp] = true
			}
		} else {
			// using list
			v.unsp = make([]OneAllAddrInp, int(outs))
			bts = len(v.unsp) * len(v.unsp[0])
			slice = *(*[]byte)(unsafe.Pointer(&reflect.SliceHeader{Data: uintptr(unsafe.Pointer(&v.unsp[0][0])), Len: bts, Cap: bts}))
			er = btc.ReadAll(rd, slice)
			if er != nil {
				return
			}
		}

		if *abort {
			return
		}

		allbal[key] = v

		if cnt_dwn == 0 {
			fmt.Print("\rLoading ", what, " - ", perc, "% complete ... ")
			perc++
			cnt_dwn = cnt_dwn_from
		} else {
			cnt_dwn--
		}
	}

	fmt.Print("\r                                                              \r")

	// all good
	res = allbal
	return
}

func save_one_map(wr *bufio.Writer, allbal map[[20]byte]*OneAllAddrBal, what string) {
	var bts, cnt_dwn_from, cnt_dwn, perc int
	var slice []byte

	what = fmt.Sprint(len(allbal), " ", what, " addresses")
	cnt_dwn_from = len(allbal) / 100

	btc.WriteVlen(wr, uint64(len(allbal)))
	for k, v := range allbal {
		wr.Write(k[:])
		btc.WriteVlen(wr, v.Value)
		if v.unspMap != nil {
			btc.WriteVlen(wr, uint64(len(v.unspMap)))
			for ii, _ := range v.unspMap {
				wr.Write(ii[:])
			}
		} else {
			btc.WriteVlen(wr, uint64(len(v.unsp)))
			bts = len(v.unsp) * len(v.unsp[0])
			slice = *(*[]byte)(unsafe.Pointer(&reflect.SliceHeader{Data: uintptr(unsafe.Pointer(&v.unsp[0][0])), Len: bts, Cap: bts}))
			wr.Write(slice)
		}

		if cnt_dwn == 0 {
			fmt.Print("\rSaving ", what, " - ", perc, "% complete ... ")
			perc++
			cnt_dwn = cnt_dwn_from
		} else {
			cnt_dwn--
		}
	}
	fmt.Print("\r                                                              \r")
}

func save_one_map32(wr *bufio.Writer, allbal map[[32]byte]*OneAllAddrBal, what string) {
	var bts, cnt_dwn_from, cnt_dwn, perc int
	var slice []byte

	what = fmt.Sprint(len(allbal), " ", what, " addresses")
	cnt_dwn_from = len(allbal) / 100

	btc.WriteVlen(wr, uint64(len(allbal)))
	for k, v := range allbal {
		wr.Write(k[:])
		btc.WriteVlen(wr, v.Value)
		if v.unspMap != nil {
			btc.WriteVlen(wr, uint64(len(v.unspMap)))
			for ii, _ := range v.unspMap {
				wr.Write(ii[:])
			}
		} else {
			btc.WriteVlen(wr, uint64(len(v.unsp)))
			bts = len(v.unsp) * len(v.unsp[0])
			slice = *(*[]byte)(unsafe.Pointer(&reflect.SliceHeader{Data: uintptr(unsafe.Pointer(&v.unsp[0][0])), Len: bts, Cap: bts}))
			wr.Write(slice)
		}

		if cnt_dwn == 0 {
			fmt.Print("\rSaving ", what, " - ", perc, "% complete ... ")
			perc++
			cnt_dwn = cnt_dwn_from
		} else {
			cnt_dwn--
		}
	}
	fmt.Print("\r                                                              \r")
}

func Save() {
	if common.FLAG.NoWallet {
		os.Remove(common.GocoinHomeDir + FILE_NAME)
		return
	}

	UpdateMapSizes()

	if !common.CFG.AllBalances.SaveOnDisk {
		os.Remove(common.GocoinHomeDir + FILE_NAME)
		return
	}

	if bytes.Equal(file_for_block[:], common.Last.Block.BlockHash.Hash[:]) {
		fmt.Println("No need to update", FILE_NAME)
		return
	}

	f, er := os.Create(common.GocoinHomeDir + FILE_NAME)
	if er != nil {
		println(er.Error())
		return
	}

	wr := bufio.NewWriter(f)
	sta := time.Now()

	wr.Write(common.Last.Block.BlockHash.Hash[:])
	btc.WriteVlen(wr, common.AllBalMinVal())
	wr.Write([]byte{byte(utxo.UtxoIdxLen)})

	save_one_map(wr, AllBalancesP2KH, "P2KH")
	save_one_map(wr, AllBalancesP2SH, "P2SH")
	save_one_map(wr, AllBalancesP2WKH, "P2WKH")
	save_one_map32(wr, AllBalancesP2WSH, "P2WSH")

	wr.Write(END_MARKER[:])
	wr.Flush()
	f.Close()
	fmt.Print("\r", FILE_NAME, " saved in ", time.Now().Sub(sta).String())
	fmt.Println()
}

const (
	MAPSIZ_FILE_NAME = "mapsize.gob"
)

var (
	WalletAddrsCount map[uint64][4]int = make(map[uint64][4]int) //index:MinValue, [0]-P2KH, [1]-P2SH, [2]-P2WSH, [3]-P2WKH
)

func UpdateMapSizes() {
	WalletAddrsCount[common.AllBalMinVal()] = [4]int{len(AllBalancesP2KH),
		len(AllBalancesP2SH), len(AllBalancesP2WKH), len(AllBalancesP2WSH)}

	buf := new(bytes.Buffer)
	gob.NewEncoder(buf).Encode(WalletAddrsCount)
	ioutil.WriteFile(common.GocoinHomeDir+MAPSIZ_FILE_NAME, buf.Bytes(), 0600)
}

func LoadMapSizes() {
	d, er := ioutil.ReadFile(common.GocoinHomeDir + MAPSIZ_FILE_NAME)
	if er != nil {
		println("LoadMapSizes:", er.Error())
		return
	}

	buf := bytes.NewBuffer(d)

	er = gob.NewDecoder(buf).Decode(&WalletAddrsCount)
	if er != nil {
		println("LoadMapSizes:", er.Error())
	}
}
