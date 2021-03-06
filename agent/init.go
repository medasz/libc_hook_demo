/*
 * @Date: 2019-12-26 20:33:42
 * @LastEditors  : u2400
 * @LastEditTime : 2020-02-02 20:58:21
 */

package agent

/*
#cgo linux LDFLAGS: -lrt
#include "../C_util/get_proc_inf.h"
#include "../C_util/util.h"
#include "../config.h"
*/
import "C"
import (
	"fmt"
	"time"
	"unsafe"
	"HIDS-agent/Go_util"
	"syscall"
	"os/signal"
	"os"
)

var data_mem *[200](* C.proc_inf)
var inf_mem *[2]C.int
var inf_chan chan Go_util.Proc_inf
var data_mem_size int

/**
 * @description: 初始化agent, 包括创建共享内存, 启动数据获取进程和数据处理进程
 * @param void
 * @return: void
 */
func Init() {
	data_mem_size = C.data_mem_size
	// wait_group := new(sync.WaitGroup)
	// wait_group.Add(1)
	inf_ptr, err := new_share_mem(C.share_mem_name, C.node_size * data_mem_size)
	if (err == -1) {
		fmt.Println("new share mem error!")
		return
	}
	defer free_share_mem(C.share_mem_name)

	inf, err := new_share_mem(C.share_mem_inf_name, C.int_size * 3)
	if (err == -1) {
		fmt.Println("new share mem inf error!")
		return
	}
	defer free_share_mem(C.share_mem_inf_name)

	inf_chan = make(chan Go_util.Proc_inf, data_mem_size)
	data_mem = (*[200](* C.proc_inf))(inf_ptr)

	//inf_mem[0] 写标志位, 当生产者(hook)对共享内存写入数据时会将该位置1
	//inf_mem[1] 共享内存采用栈的方式进行管理, 该位指向当前空闲内存的索引
	inf_mem = (*[2]C.int)(inf)
	inf_mem[0] = C.int(0)
	inf_mem[1] = C.int(0)
	go data_setter()
	// go data_getter()

	fmt.Println("agent is running!")
	wait_exit()
	// wait_group.Wait()
}

/**
 * @description: 轮询共享内存, 当写标识为为1时, 读取共享内存的全部数据, 放入管道中, 并将写标志位制0
 * @param void
 * @return: void
 */
func data_setter() {
	for {
		if inf_mem[0] > 0 {
			for i := (inf_mem[1] - 1); i >= 0; i-- {
				C_inf := data_mem[i]
				go_inf := cs_to_gs(C_inf)
				inf_chan <- go_inf
			}
			inf_mem[0] = 0
			inf_mem[1] = 0
		}
		time.Sleep(100000);
	}
}

func cs_to_gs(cs *C.proc_inf) Go_util.Proc_inf {
	gs := Go_util.Proc_inf{}
	gs.Path = C.GoString(cs.Path)
	// gs.ENV = C.GoString(cs.ENV)
	gs.Cmdline = C.GoString(cs.Cmdline)
	gs.Pcmdline = C.GoString(cs.Pcmdline)
	gs.PPath = C.GoString(cs.PPath)
	gs.Name = C.GoString(cs.Name)
	gs.Pid = int(cs.Pid)
	gs.PPid = int(cs.PPid)
	gs.Pgid = int(cs.Pgid)
	gs.Uid = int(cs.Uid)
	gs.Euid = int(cs.Euid)
	gs.Gid = int(cs.Gid)
	gs.Egid = int(cs.Egid)
	gs.Sid = int(cs.Sid)
	gs.Owner_uid = int(cs.Owner_uid)
	gs.Owner_gid = int(cs.Owner_gid)
	gs.Create_time = int(cs.Create_time)
	gs.Modify_time = int64(cs.Modify_time)
	gs.Start_time = int64(cs.Start_time)
	gs.Run_time = int64(cs.Run_time)
	gs.Sys_time = int64(cs.Sys_time)
	// Fd = 
	return gs
}

/**
 * @description: 获取Pid并对获取Pid的进程数据进行补全
 * @param void
 * @return: void
 */
// func data_getter() {
// 	for {
// 		Pid := <- inf_chan
// 		inf := Go_util.Get_proc_inf(Pid)
// 		json_inf, err := json.Marshal(inf)
// 		if err != nil {
// 			Go_util.Catch_error(err)
// 		}
// 		fmt.Println(string(json_inf))
// 	}
// }

/**
 * @description: 传入内存共享的名字和大小, 返回新建的或打开的内存共享区域.
 * @param name string, size int
 * @return: unsafe.Pointer,int
 */
func new_share_mem(name string, size int) (unsafe.Pointer,int) {
	var ptr unsafe.Pointer
	var err0, err1 error
	ptr, err0 = C.new_share_mem(C.CString(name), C.ulong(size))
	if err0 != nil {
		ptr, err1 = C.open_share_mem(C.CString(name), C.ulong(size))
		if err1 != nil {
			fmt.Println("err0:",err0)
			fmt.Println("err1:",err1)
			return nil,-1
		}
	}
	os.Chmod( "/dev/shm/"+ name, 0666)
	return ptr,0
}

func free_share_mem(name string) {
	fmt.Println("agent log: free mem share")
	C.free_share_mem(C.CString(name))
}

func wait_exit() {
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, os.Kill, syscall.SIGUSR1, syscall.SIGUSR2)
	_ = <-c
	fmt.Println("proc exit!")
	C.free_share_mem(C.CString(C.share_mem_name))
	C.free_share_mem(C.CString(C.share_mem_inf_name))
} 