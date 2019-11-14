package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"github.com/NextDesignSolutions/njclient"
	"log"
	"os"
)

func main() {
	log.SetOutput(os.Stdout)

	var rnw = flag.Bool("readNotWrite", true, "Read Not Write")
	var addr = flag.Uint64("addr", 0xc0000000, "address")
	var size = flag.Uint64("size", 0x4, "size of read data or size of write data element")
	var bufferable = flag.Bool("bufferable", true, "Bufferable Cache Attribute")
	var modifiable = flag.Bool("modifiable", true, "Modifiable Cache Attribute")
	var read_alloc = flag.Bool("read_alloc", true, "Read alloc Cache Attribute")
	var write_alloc = flag.Bool("write_alloc", true, "Write alloc Cache Attribute")
	var incr_mode = flag.Bool("incr_mode", true, "increment or fixed burst mode")
	var board_id = flag.String("board", "", "board ID")
	var fpga_index = flag.Int("fpga", 0x0, "FPGA index")
	var api_uri = flag.String("uri", "http:/127.0.0.1:19080", "nextjtag server API, default=http://127.0.0.1:19080")
	var num_columns = flag.Int("num_columns", 1, "number of columns to display")
	var column_size = flag.Int("column_size", 4, "column size in bytes. Options: 1,2,4,8")
	var write_data = flag.Uint64("write_data", 0, "write data, can be up to 64-bits")
	flag.Parse()

	mod := *addr % 4
	if mod != 0 {
		log.Fatalln("address must be aligned to access size: 4 bytes")
	}

	config := &njclient.Config{APIVersion: "v1"}
	client := njclient.NewClient(config, *api_uri)
	_, err := client.GetServerVersion()
	if err != nil {
		log.Fatalln("unable to get version: ", err)
	} //else {
	//  fmt.Println("minor: ", v.Minor)
	//  fmt.Println("major: ", v.Major)
	//  fmt.Println("sha1: ", v.Sha1)
	//  fmt.Println("version: ", v.Version)
	//}
	bs := client.BoardService
	err = bs.QueryBoards()
	if err != nil {
		log.Fatalln("unable to query boards: ", err)
	}
	var b *njclient.Board = nil
	if *board_id == "" {
		b, err = bs.GetSomeBoard()
		if err != nil {
			log.Fatalln(err)
		}
	} else {
		b, err = bs.GetBoard(*board_id)
		if err != nil {
			log.Fatalln(err)
		}
	}
	err = b.Init()
	if err != nil {
		log.Fatalln("failed to initialize board with key ", b.Key, err)
	}
	fserve := b.FpgaService
	err = fserve.QueryFpgas()
	if err != nil {
		log.Fatalln("failed to query fpgas for board ", b.Key)
	}
	fpga, err := fserve.GetFpga(*fpga_index)
	if err != nil {
		log.Fatalln(err)
	}

	attr := njclient.NewAxiCacheAttributes(*bufferable, *modifiable, *read_alloc, *write_alloc)
	if attr == nil {
		log.Fatalln("failed to create AxiCacheAttributes!, ", err)
	}
	count := int(((*size + 3) / 4))
	opts := njclient.NewAxiTransactionOptions(*incr_mode, count)
	if opts == nil {
		log.Fatalln("failed to create AxiTransactionOptions!, ", err)
	}
	// FIXME: add write support

	var data []uint32
	if *size == 4 {
		data = append(data, (uint32)((*write_data)&0xffffffff))
	} else if *size == 8 {
		data = append(data, (uint32)((*write_data)&0xffffffff))
		data = append(data, (uint32)(((*write_data)>>32)&0xffffffff))
	} else {
		log.Fatalln("sorry, only supporting write lengths of 4/8 bytes")
	}

	t := njclient.NewAxiTransaction(*addr, *rnw, opts, attr, &data)
	if t == nil {
		log.Fatalln("failed to create AxiTransaction, ", err)
	}
	axih := fpga.AxiService.GetAvailableAxiHandle()
	if axih == nil {
		log.Fatalln("failed to get an axi handle")
	} else {
		result, err := axih.IssueTransaction(t)
		if err != nil {
			log.Fatalln("Axi transaction failed, ", err)
		} else {
			if result.Response != "OKAY" && result.Response != "EXOKAY" {
				log.Fatalln("received error from hardware, ", result.Response)
			}
			// read case, display data
			if *rnw == true {
				if result.Value == nil {
					log.Fatalln("failed to extract read data")
				} else {
					buf := new(bytes.Buffer)
					err := binary.Write(buf, binary.LittleEndian, *result.Value)
					if err != nil {
						log.Fatalln("binary.Write failed:", err)
					}
					rbuf := bytes.NewReader(buf.Bytes())

					if *column_size == 1 {
						check := make([]uint8, buf.Len())
						err = binary.Read(rbuf, binary.LittleEndian, &check)
						if err != nil {
							log.Fatalln("binary.Read failed: ", err)
						}

						current_col := 0
						var msg string = ""
						for i, b := range check {
							if current_col >= *num_columns {
								msg += "\n"
								current_col = 0
							}
							if current_col == 0 {
								offset := i
								msg += fmt.Sprintf("%012x:", *addr+uint64(offset))
							}
							msg += fmt.Sprintf(" %02x", b)
							current_col += 1
						}
						fmt.Printf("%s\n", msg)
					} else if *column_size == 2 {
						check := make([]uint16, (buf.Len()+1)/2)
						err = binary.Read(rbuf, binary.LittleEndian, &check)
						if err != nil {
							log.Fatalln("binary.Read failed: ", err)
						}
						current_col := 0
						var msg string = ""
						for i, b := range check {
							if current_col >= *num_columns {
								msg += "\n"
								current_col = 0
							}
							if current_col == 0 {
								offset := i
								msg += fmt.Sprintf("%012x:", *addr+uint64(offset))
							}
							msg += fmt.Sprintf(" %04x", b)
							current_col += 1
						}
						fmt.Printf("%s\n", msg)
					} else if *column_size == 4 {
						check := make([]uint32, (buf.Len()+3)/4)
						err = binary.Read(rbuf, binary.LittleEndian, &check)
						if err != nil {
							log.Fatalln("binary.Read failed: ", err)
						}
						current_col := 0
						var msg string = ""
						for i, b := range check {
							if current_col >= *num_columns {
								msg += "\n"
								current_col = 0
							}
							if current_col == 0 {
								offset := i
								msg += fmt.Sprintf("%012x:", *addr+uint64(offset))
							}
							msg += fmt.Sprintf(" %08x", b)
							current_col += 1
						}
						fmt.Printf("%s\n", msg)
					} else if *column_size == 8 {
						check := make([]uint64, (buf.Len()+7)/8)
						err = binary.Read(rbuf, binary.LittleEndian, &check)
						if err != nil {
							log.Fatalln("binary.Read failed: ", err)
						}
						current_col := 0
						var msg string = ""
						for i, b := range check {
							if current_col >= *num_columns {
								msg += "\n"
								current_col = 0
							}
							if current_col == 0 {
								offset := i
								msg += fmt.Sprintf("%012x:", *addr+uint64(offset))
							}
							msg += fmt.Sprintf(" %016x", b)
							current_col += 1
						}
						fmt.Printf("%s\n", msg)
					} else {
						log.Fatalln("column_size provided not supported")
					}

				}
			}
		}
	}
}
