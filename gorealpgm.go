package main

import (
	"fmt"
	"bufio"
	"os/exec"
	"github.com/hypersleep/easyssh"
	"strings"
	"os"
	"database/sql"
	_ "github.com/lib/pq"
	"net/smtp"
	"bytes"
	"log"
	)

// declare  a string which will mail the test results 
// declare a variable to test whether standby activation need to be done


//function to mail

func send_mail ( gpm map[string]string, msg *string) int {
var ( subject string
	body string
	)
mail_to := gpm["mail_list"]
ml,err := smtp.Dial("mailsyshubprd05.lss.emc.com:25")

if err != nil {
	log.Fatal(err)
		}

ml.Mail("BigData@emc.com")
ml.Rcpt(mail_to)

// Connecting to mail server ?.
  wc, err_m := ml.Data()
      if err_m != nil {
                log.Fatal(err_m)
        }
        defer wc.Close()
subject = "GPDB BDL Instance" + gpm["mdw"] + "status Alert" 
body = *msg
message := bytes.NewBufferString(fmt.Sprintf("subject: %s\r\n\r\n%s", subject , body))

     if _, err_n := message.WriteTo(wc); err != nil {
                log.Fatal(err_n)
        }
return 0
}










//Function to ping mdw from external host and segment hosts starts here
func ping_check( gpm map[string]string , msg *string) int {

//#### ping from current host , ping from smdw , ping from segs
//var srv string
srv := gpm["mdw"]

cmd := "pinc -c 1" + srv

_,err := exec.Command(cmd).Output()
if err != nil { 
	*msg += "ping from host.com to " + srv + "failed" 
	return 1
//	os.Exit() 
	}
*msg += "ping from host.com to " + srv + "succeeded" 

for key,_ := range gpm { 
	if key == "sdw" { 
				s := strings.Split(gpm[key],",")
				for i,_ := range s {
							ssh_rem := &easyssh.MakeConfig {
												        User : "gpadmin",
        												         Server : srv,
        												       Key : "/home/gpadmin/.ssh/id_rsa",
        												       Port : "22",
        												  }
				_,err := ssh_rem.Run(cmd)
				if err != nil { *msg += "ping or ssh into" + s[i] + "failed" 
						return 1
//						os.Exit() 
						}
				if err == nil {  *msg += "ping or ssh into" + s[i] + "succeeded"}
			}
}
}
return 0
}
// function to ping mdw from external and segment hosts ends here




//Function to run db query against mdw from external host starts here

func db_qry_chk(gpm map[string]string , msg *string, call_frm string) int {
var ( db_hst string
	db_nme string
)
if call_frm == "mdw" {
db_hst = gpm["mdw"]
db_nme = gpm["db"]
}

if call_frm == "smdw" {
db_hst = gpm["smdw"]
db_nme = gpm["db"]
}

if call_frm == "smdw_vip" {
db_hst = gpm["vip"]
db_nme = gpm["db"]
}


// parametrize db name in the samptest file . With anothe key value pair . like db:db_name
db_conn,err := sql.Open("postgres","postgres://gpadmin:Gpadmin1@db_hst/db_nme?sslmode=disable")
if err != nil {
		*msg += "connecting database" + db_nme +  " on host " + db_hst + " failed" 
		return 1
		}

_,err = db_conn.Query("select * from pg_stat_activity")

if err != nil {
		*msg += "query against Database" + db_nme + "on host " + db_hst + "failed with error :" 
		return 1
		}
return 0
}


// Function to run db query against mdw ends here



//Function to check VIP starts here 

func check_vip(gpm map[string]string , msg *string) int {

vip := gpm["vip"]
cmd :=  "ping -c 1 " + vip

_,err := exec.Command(cmd).Output()
if err == nil {
		*msg += "VIP still reachable" 
                return 1
}

*msg += "VIP  unreachable"
return 0
}
// function to check VIP ends here


//function to flip vip and initialize standby starts here


func init_standby(gpm map[string]string , msg *string) int {
var ret int
smdw :=gpm["smdw"]
vip := gpm["vip"]
data_dir := gpm["datadir"]

gpinit_cmd := "gpactivatestandby -d " + data_dir 
gpinit_validate := "gpstate -f |  grep 'Standby status' " 
vip_cmd := "ifconfig eth0:0 " + vip + "up"
chk_vip := "ping -c 1" + vip
ssh_rem := &easyssh.MakeConfig {
                                                                                                        User : "gpadmin",
                                                                                                                 Server : smdw,
                                                                                                               Key : "/home/gpadmin/.ssh/id_rsa",
                                                                                                               Port : "22",
                                                                                                          }
_,err_init := ssh_rem.Run(gpinit_cmd)
_,err_vali := ssh_rem.Run(gpinit_validate)
if err_init != nil  ||  err_vali != nil { *msg += "activating Standby Failed"
	return 1
}



ret = db_qry_chk(gpm,msg,"smdw")
if ret != 0 {
                return 1
                }

_,err_vip := ssh_rem.Run(chk_vip)

if err_vip == nil { 
		*msg += "VIP still reachable after activating Standby . It should be deactivated on Master."
// create a function to disable VIP 	
		//check VIP again 
		_,err_vip1 := ssh_rem.Run(chk_vip)
if err_vip1 == nil { *msg += "VIP still reachable after activating Standby and After couple of Attempts. Database is Up on Standby right now please Check the VIP"
			return 1
		}
		}

_,err_act_vip := ssh_rem.Run(vip_cmd)
if err_act_vip !=nil {  *msg += "VIP Actiavtion failed on Standby"
		return 1
		}
ret = db_qry_chk(gpm,msg,"smdw_vip")
if ret != 0 {
                return 1
                }
// Revise this function once more and see what could be added here
//pick up from here 

return 0
}
// function to init standby ends here


func main() {

var gpmap map[string]string
var s1 []string
file,_ := os.Open(os.Args[1])
gpmap = make(map[string]string)
scn := bufio.NewScanner(file)
scn.Split(bufio.ScanLines)


for scn.Scan() {

fmt.Println(scn.Text())
s1 = strings.Split(scn.Text(),":")
gpmap[s1[0]] = s1[1]

		}

ret_msg :=""
chi := ping_check(gpmap,&ret_msg)

if chi != 0 {
	send_mail(gpmap,&ret_msg)
	os.Exit(1)
	}

chi = db_qry_chk(gpmap,&ret_msg,"mdw")
if chi != 0 {
		send_mail(gpmap,&ret_msg)
                os.Exit(1)
                }


fmt.Println(chi)
fmt.Println(ret_msg)

chi = check_vip(gpmap,&ret_msg)
if chi != 0 {
// add a step here to check where VIP is actie switch it off.
send_mail(gpmap,&ret_msg)
                os.Exit(1)
                }

chi = init_standby(gpmap,&ret_msg)

if chi != 0 {send_mail(gpmap,&ret_msg)
                os.Exit(1)
                }

}
