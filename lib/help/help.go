package help

import (
	"archive/zip"
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/howeyc/gopass"
	"github.com/hypersleep/easyssh"
	"github.com/xshellinc/tools/lib/sudo"
	pb "gopkg.in/cheggaaa/pb.v1"
)

type Iface struct {
	Name         string
	HardwareAddr string
	Ipv4         string
}

const (
	SshExtendedCommandTimeout = 300
	SshCommandTimeout         = 30

	DefaultSshPort = "22"
)

func UserHomeDir() string {
	if runtime.GOOS == "windows" {
		home := os.Getenv("HOMEDRIVE") + os.Getenv("HOMEPATH")
		if home == "" {
			home = os.Getenv("USERPROFILE")
		}
		return home
	}
	return os.Getenv("HOME")
}

func Separator() string {
	s := string(filepath.Separator)
	return s
}

func ExecSudo(cb sudo.PasswordCallback, cbData interface{}, script ...string) (string, error) {
	out, eut, err := sudo.Exec(cb, cbData, script...)
	LogCmdErrors(string(out), string(eut), err, script...)
	if err != nil {
		return string(append(out, eut...)), err
	}
	return string(out), err
}

func ExecCmd(cmdName string, cmdArgs []string) (string, error) {
	cmd := exec.Command(cmdName, cmdArgs...)
	cmdOutput := &bytes.Buffer{}
	cmdStdErr := &bytes.Buffer{}
	cmd.Stdout = cmdOutput
	cmd.Stderr = cmdStdErr
	err := cmd.Run()

	LogCmdErrors(cmdOutput.String(), cmdStdErr.String(), err, cmd.Args...)
	if err != nil {
		return string(append(cmdOutput.Bytes(), cmdStdErr.Bytes()...)), err
	}
	return cmdOutput.String(), err
}

func LogCmdErrors(out, eut string, err error, args ...string) {
	if err != nil {
		log.Error("Error while executing: `", args, "` error msg: `", eut,
			"` go error:", err.Error())
		log.Error("Output:", out)
	}
}

func CreateFile(path string) {
	// detect if file exists
	var _, err = os.Stat(path)
	// create file if not exists
	if os.IsNotExist(err) {
		var file, err = os.Create(path)
		logError(err)
		defer file.Close()
	}
}

func CreateDir(path string) error {
	err := os.MkdirAll(path, os.ModePerm)
	if err != nil {
		log.Error("Error creating dir: ", path, " error msg:", err.Error())
		return err
	}
	return nil
}

func DeleteFile(path string) error {
	log.Debug("deleting file:", path)
	err := os.Remove(path)
	if err != nil {
		logError(err)
		return err
	}
	return nil
}

func WriteFile(path string, content string) {
	// open file using READ & WRITE permission
	var file, err = os.OpenFile(path, os.O_RDWR, 0644)
	logError(err)
	defer file.Close()
	// write some text to file
	_, err = file.WriteString(content)
	logError(err)
	// save changes
	err = file.Sync()
	logError(err)
	err = file.Sync()
	logError(err)
}

func DeleteDir(dir string) error {
	d, err := os.Open(dir)
	log.Debug("DeleteDir func():", "removing dir:", dir)
	if err != nil {
		logError(err)
		return err
	}
	defer d.Close()
	names, err := d.Readdirnames(-1)
	if err != nil {
		logError(err)
		return err
	}
	for _, name := range names {
		err = os.RemoveAll(filepath.Join(dir, name))
		if err != nil {
			logError(err)
			return err
		}
	}
	return nil
}

func logError(err error) {
	if err != nil {
		log.Error(err.Error())
	}
}

// DownloadFromUrl downloads target file to destination folder
// create destination dir if does not exist
// download file if does not already exist
// shows progress bar
func DownloadFromUrl(url, destination string) (string, error) {
	var (
		timeout time.Duration = time.Duration(0)
		client  http.Client   = http.Client{Timeout: timeout}
	)
	//tokenize url
	tokens := strings.Split(url, "/")
	//obtain file name
	fileName := tokens[len(tokens)-1]

	// check maybe downloaded file exists and corrupted
	fullFileName := filepath.Join(destination, fileName)
	if _, err := os.Stat(fullFileName); !os.IsNotExist(err) {

		downloadedFileLength, _ := GetFileLength(fullFileName)
		sourceFileLength, _ := GetHTTPFileLength(url)

		if sourceFileLength != downloadedFileLength && sourceFileLength != 0 {
			fmt.Printf("[+] Delete corrupted cached file %s\n", fullFileName)
			DeleteFile(fullFileName)
		}
		// otherwise file has correct length
	}

	fmt.Printf("[+] Downloading %s from %s to %s\n", fileName, url, destination)

	//target file does not exist
	if _, err := os.Stat(fmt.Sprintf("%s/%s", destination, fileName)); os.IsNotExist(err) {
		//create destination dir
		CreateDir(destination)
		//create file
		output, err := os.Create(fmt.Sprintf("%s/%s", destination, fileName))
		if err != nil {
			log.Error("[-] Error creating file ", destination, fileName)
			return "", err
		}
		defer output.Close()
		response, err := client.Get(url)
		if err != nil {
			log.Error("[-] Error while downloading file ", url)
			return "", err
		}
		defer response.Body.Close()
		bar := pb.New64(response.ContentLength)
		bar.ShowBar = false
		bar.SetWidth(100)
		bar.Start()
		prd := bar.NewProxyReader(response.Body)
		totalCount, err := io.Copy(output, prd)
		if err != nil {
			log.Error("error while copying ", err.Error())
			return "", err
		}
		log.Debug("Total number of bytes read: ", totalCount)
	} else {
		fmt.Printf("[+] File exist %s%s\n", destination, fileName)
	}
	fmt.Println("\n[+] Done")
	return fileName, nil
}

// DownloadFromUrl downloads target file to destination folder,
// creates destination dir if does not exist
// download file if does not already exist
// shows progress bar
func DownloadFromUrlAsync(url, destination string, readBytesChannel chan int64, errorChan chan error) (string, int64, error) {
	var (
		timeout  time.Duration = time.Duration(0)
		client   http.Client   = http.Client{Timeout: timeout}
		fileName               = "latest"
		length   int64
	)
	response, err := client.Get(url)
	if err != nil {
		log.Error("Error while downloading file from:", url, " error msg:", err.Error())
		return "", 0, err
	}
	contentDisposition := response.Header.Get("Content-Disposition")
	_, params, err := mime.ParseMediaType(contentDisposition)
	if err != nil {
		tokens := strings.Split(url, "/")
		fileName = tokens[len(tokens)-1]
		log.Error("Error parsing media type ", "error msg:", err.Error())
	} else {
		fileName = params["filename"]
	}
	length = response.ContentLength
	// check maybe downloaded file exists
	fullFileName := filepath.Join(destination, fileName)
	if _, err := os.Stat(fullFileName); !os.IsNotExist(err) {
		downloadedFileLength, _ := GetFileLength(fullFileName)
		sourceFileLength, _ := GetHTTPFileLength(url)
		if sourceFileLength > downloadedFileLength && sourceFileLength != 0 {
			log.Debug("Missing bytes. Resuming download")
			go resumeDownloadAsync(fullFileName, url, readBytesChannel, errorChan)
		} else {
			//report full length
			readBytesChannel <- downloadedFileLength
			//close channel
			close(readBytesChannel)
			close(errorChan)
			return fileName, downloadedFileLength, nil
		}
	} else { //file does not exist, download file
		CreateDir(destination)
		//create file
		output, err := os.Create(fmt.Sprintf("%s/%s", destination, fileName))
		if err != nil {
			log.Error("Error creating file ", destination, fileName)
			return "", 0, err
		}
		// ASYNC part. Download file in background and send read bytes into the readBytesChannel channel
		go func(resp *http.Response, ch chan int64, output *os.File, errorChan chan error) {
			defer response.Body.Close()
			defer close(ch)
			defer output.Close()
			defer close(errorChan)
			prd := NewHttpProxyReader(response.Body, func(n int, err error) {
				ch <- int64(n)
				if err != nil {
					log.Error("error occured, sending error down the channel")
					errorChan <- err
					return
				}
			})

			totalCount, err := io.Copy(output, prd)
			if err != nil {
				log.Error("error while copying ", err.Error())
				errorChan <- err
				return
			}
			log.Debug("Total number of bytes read: ", totalCount)

		}(response, readBytesChannel, output, errorChan)
	}
	return fileName, length, nil
}

func DownloadFromUrlSilent(url, destination string) (string, error) {
	var (
		timeout time.Duration = time.Duration(0)
		client  http.Client   = http.Client{Timeout: timeout}
	)
	//tokenize url
	tokens := strings.Split(url, "/")
	//obtain file name
	fileName := tokens[len(tokens)-1]

	// check maybe downloaded file exists and corrupted
	fullFileName := filepath.Join(destination, fileName)
	if _, err := os.Stat(fullFileName); !os.IsNotExist(err) {

		downloadedFileLength, _ := GetFileLength(fullFileName)
		sourceFileLength, _ := GetHTTPFileLength(url)

		if sourceFileLength != downloadedFileLength && sourceFileLength != 0 {
			log.Debug("Delete corrupted cached file %s\n", fullFileName)
			DeleteFile(fullFileName)
		}
		// otherwise file has correct length
	}

	log.Debug("Downloading %s from %s to %s\n", fileName, url, destination)

	//target file does not exist
	if _, err := os.Stat(fmt.Sprintf("%s/%s", destination, fileName)); os.IsNotExist(err) {
		//create destination dir
		CreateDir(destination)
		//create file
		output, err := os.Create(fmt.Sprintf("%s/%s", destination, fileName))
		if err != nil {
			log.Error("[-] Error creating file ", destination, fileName)
			return "", err
		}
		defer output.Close()
		response, err := client.Get(url)
		if err != nil {
			log.Error("[-] Error while downloading file ", url)
			return "", err
		}
		defer response.Body.Close()
		bar := pb.New64(response.ContentLength)
		bar.ShowBar = false
		prd := bar.NewProxyReader(response.Body)
		totalCount, err := io.Copy(output, prd)
		if err != nil {
			log.Error("error while copying ", err.Error())
			return "", err
		}
		log.Debug("Total number of bytes read: ", totalCount)
	} else {
		log.Debug("File exist %s%s\n", destination, fileName)
	}
	log.Debug("\nDone")
	return fileName, nil
}

func resumeDownloadAsync(dst, url string, ch chan int64, errorChan chan error) {
	log.Debug("resuming download to ", dst)
	//resolve redirects to final url
	defer close(ch)
	defer close(errorChan)
	final_url, err := GetFinalUrl(url)
	if err != nil {
		log.Error("http.Get :", err.Error())
	}
	log.Debug("Final resolved URL:", final_url)
	local_length, err := GetFileLength(dst)
	if err != nil {
		log.Error("error getting file length from:", dst, " error msg:", err.Error())
		return
	}
	remote_length, err := GetHTTPFileLength(final_url)
	if err != nil {
		log.Error("error getting remote file length from:", final_url, "error msg:", err.Error())
		return
	}
	log.Debug("Current file size: ", local_length, " remote file length:", remote_length)
	if local_length < remote_length {
		log.Debug("Downloading : ", strconv.FormatInt(remote_length-local_length-1, 10), " bytes")
		//send actual size of the file
		ch <- local_length
		client := &http.Client{}
		req, err := http.NewRequest("GET", final_url, nil)
		if err != nil {
			log.Error("error creating GET request to ", final_url, " error msg:", err.Error())
			return
		}
		range_header := "bytes=" + strconv.FormatInt(local_length, 10) + "-"
		//"-" + strconv.FormatInt(remote_length-1, 10) + "/" + strconv.FormatInt(remote_length, 10)
		req.Header.Add("Range", range_header)
		log.Debug("Adding Range header:", range_header)
		resp, err := client.Do(req)
		defer resp.Body.Close()
		if err != nil {
			log.Error("error making GET request to ", final_url, " error msg:", err.Error())
			return
		}
		log.Debug("Received content length:", resp.ContentLength)
		if resp.StatusCode != http.StatusPartialContent {
			log.Debug("HTTP status code:", resp.StatusCode)
			log.Error("Server does not support Range header, cannot resume download.")
			fmt.Println("\n[-] Server does not support Range header. Cannot resume download. Delete your old file and re-run your command again")
			return
		}

		prd := NewHttpProxyReader(resp.Body, func(n int, err error) {
			ch <- int64(n)
			if err != nil {
				errorChan <- err
			}
		})
		output, err := os.OpenFile(dst, os.O_APPEND|os.O_WRONLY, 0600)
		defer output.Close()
		if err != nil {
			log.Error("error opening file ", "error msg:", err.Error())
		}
		totalCount, err := io.Copy(output, prd)
		if err != nil {
			log.Error("error while copying ", err.Error())
		}
		log.Debug("Total number of bytes read: ", totalCount)
	}
}

func GetFileLength(path string) (int64, error) {
	var file, err = os.OpenFile(path, os.O_RDONLY, 0644)
	if err != nil {
		return 0, errors.New("File not found " + path)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return 0, errors.New("Can't fetch file info " + path)
	}

	return stat.Size(), nil

}

func GetHTTPFileLength(url string) (int64, error) {
	var (
		timeout time.Duration = time.Duration(0)
		client  http.Client   = http.Client{Timeout: timeout}
	)

	response, err := client.Get(url)
	if err != nil {
		return 0, errors.New("Can't retrieve length of http source " + url)
	}

	return response.ContentLength, nil
}

func GetFinalUrl(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	return resp.Request.URL.String(), nil
}

func DownloadFromUrlWithAttempts(url, destination string, retries int) (string, error) {
	var (
		err      error
		filename string
	)
	for i := 1; i <= retries; i++ {
		fmt.Printf("[+] Attempting to download. Trying %d out of %d \n", i, retries)
		filename, err = DownloadFromUrl(url, destination)
		if err == nil {
			break
		} else {
			DeleteFile(filepath.Join(destination, filename))
		}
	}
	if err != nil {
		fmt.Printf("[-] Could not download from url:%s \n", url)
		fmt.Printf("[-] Reported error message:%s\n", err.Error())
		return "", err
	}
	return filename, nil

}

func DownloadFromUrlWithAttemptsAsync(url, destination string, retries int, wg *sync.WaitGroup) (string, *pb.ProgressBar, error) {
	var (
		err                     error
		filename                string
		destinationFileNameFull string
		length                  int64
		readBytesChannel        = make(chan int64, 10000)
		errorChan               = make(chan error, 1)
	)
	bar := pb.New64(0)
	bar.ShowBar = false
	bar.SetUnits(pb.U_BYTES)
	wg.Add(1)
	go func(ch chan int64, wg *sync.WaitGroup, errorChan chan error, filename *string, url string) {
		// complete task after closing channel
		defer wg.Done()
		for {
			select {
			case n, ok := <-ch:
				if !ok {
					ch = nil
					log.Debug("Bytes chan is closed")
				}
				bar.Add64(n)
			case err, ok := <-errorChan:
				if !ok {
					errorChan = nil
					log.Debug("Error chan is closed")
				}
				if err != nil {
					// compare length of files, if length is the same - file is downloaded
					fileLength, _ := GetFileLength(*filename)
					urlFileLength, _ := GetHTTPFileLength(url)
					fmt.Println(*filename, url)
					fmt.Println(fileLength, urlFileLength)
					if fileLength != urlFileLength {
						fmt.Println("[-] Error occured with error message:", err.Error())
						log.Error("Error occured while downloading remote file ", "error msg:", err.Error())
						os.Exit(1)
					}
				}
			}
			if ch == nil && errorChan == nil {
				break
			}

		}

	}(readBytesChannel, wg, errorChan, &destinationFileNameFull, url)

	if filename, length, err = DownloadFromUrlAsync(url, destination, readBytesChannel, errorChan); err != nil {
		fmt.Printf("[-] Could not download from url:%s \n", url)
		fmt.Printf("[-] Reported error message:%s\n", err.Error())
		close(readBytesChannel)
		close(errorChan)
		return "", nil, err
	} else {
		bar.Total = length
	}

	destinationFileNameFull = path.Join(destination, filename)

	return filename, bar, nil

}

func DownloadFromUrlWithAttemptsSilent(url, destination string, retries int) (string, error) {
	var (
		err      error
		filename string
	)
	for i := 1; i <= retries; i++ {
		log.Debug("Attempting to download. Trying %d out of %d \n", i, retries)
		filename, err = DownloadFromUrlSilent(url, destination)
		if err == nil {
			break
		} else {
			DeleteFile(filepath.Join(destination, filename))
		}
	}
	if err != nil {
		log.Debug("Could not download from url:%s \n", url)
		log.Debug("Reported error message:%s\n", err.Error())
		return "", err
	}
	return filename, nil

}

func Unzip(src, dest string) error {
	fmt.Printf("[+] Unzipping %s to %s\n", src, dest)
	tokens := strings.Split(src, "/")
	fileName := tokens[len(tokens)-1]
	// create destination dir with 0777 access rights
	CreateDir(dest)

	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}

	var total64 int64
	for _, file := range r.File {
		total64 += int64(file.UncompressedSize64)
	}
	bar := pb.New64(total64).SetUnits(pb.U_BYTES)
	bar.ShowBar = false
	bar.SetMaxWidth(80)
	bar.Prefix(fmt.Sprintf("[+] Unzipping %-15s", fileName))
	for _, f := range r.File {
		bar.Start()
		rc, err := f.Open()
		if err != nil {
			fmt.Println("[-] ", err.Error())
			return err
		}
		defer rc.Close()
		fpath := filepath.Join(dest, f.Name)
		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, f.Mode())
		} else {
			var fdir string
			if lastIndex := strings.LastIndex(fpath, string(os.PathSeparator)); lastIndex > -1 {
				fdir = fpath[:lastIndex]
			}
			err = os.MkdirAll(fdir, f.Mode())
			if err != nil {
				fmt.Println("[-] ", err.Error())
				return err
			}
			dst_f, err := os.OpenFile(
				fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				fmt.Println("[-] ", err.Error())
				return err
			}
			defer dst_f.Close()
			copied, err := io.Copy(dst_f, rc)
			if err != nil {
				fmt.Println("[-] ", err.Error())
				return err
			}
			bar.Add64(copied)
		}
	}
	bar.Finish()
	time.Sleep(time.Second * 2)
	fmt.Print("[+] Done\n")
	return nil
}

func AppendToFile(s, target string) {
	fmt.Printf("[+] Appending %s to %s \n", s, target)
	fileHandle, err := os.OpenFile(target, os.O_APPEND|os.O_RDWR, 0660)
	if err != nil {
		fmt.Printf("[-] Error while appending:%s ", err.Error())
	}
	n, err := fileHandle.WriteString(s)
	fileHandle.Close()
	log.Debug("%d bytes written \n", n)
}

func AppendRewrite(s, target string) {
	var buf bytes.Buffer
	file, err := os.Open(target)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		buf.WriteString(scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
	WriteToFile(buf.String()+s, target)
}

func WriteToFile(s, target string) error {
	fileHandle, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0660)
	if err != nil {
		fmt.Printf("[-] Error while writing:%s ", err.Error())
		return err
	}
	defer fileHandle.Close()
	_, err = fileHandle.WriteString(s)
	if err != nil {
		fmt.Println("[-] Error while writing to file:", err.Error())
		return err
	}
	return nil
}

func LocalIfaces() (i []Iface) {
	ifaces, err := net.Interfaces()
	if err != nil {
		fmt.Println("[-] Error while getting hardware ifaces:", err.Error())
		return nil
	}
	for _, iface := range ifaces {
		var (
			ip   net.IP
			face Iface
		)
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if !ip.IsLoopback() {
				if ip.To4() != nil {
					face.Ipv4 = ip.To4().String()
					face.HardwareAddr = iface.HardwareAddr.String()
					face.Name = iface.Name
					i = append(i, face)
				}
			}
		}
	}
	return i
}
func GetIface() Iface {
	ifaces := LocalIfaces()
	return ifaces[0]
}

func StreamEasySsh(ip, user, password, port, key, command string, timeout int) (chan string, chan string, chan bool, error) {
	ssh := &easyssh.MakeConfig{
		User:     user,
		Password: password,
		Port:     port,
		Server:   ip,
		Key:      key,
	}

	return ssh.Stream(fmt.Sprintf("sudo %s", command), timeout)
}

func ScpWPort(src, dst, ip, port, user, password string) error {
	ssh := &easyssh.MakeConfig{
		User:     user,
		Password: password,
		Port:     port,
		Server:   ip,
		Key:      "~/.ssh/id_rsa.pub",
	}

	fName := FileName(src)

	err := ssh.Scp(src, fName)
	if err != nil {
		fmt.Println("[-] Error uploading file:", err.Error())
		return err
	} else {
		fmt.Println("\n[+] Successfully uploaded file : ", fName)
		_, err := RunSshWithTimeout(ssh, fmt.Sprintf("sudo mv ~/%s %s", fName, dst), SshExtendedCommandTimeout)
		if err != nil {
			fmt.Println("[+] Error moving file: ", err.Error())
			return err
		}
	}

	return nil
}

func Scp(src, dst, ip, user, password string) {
	ScpWPort(src, dst, ip, "22", user, password)
}

func ScpSilent(src, dst, ip, user, password string) error {
	ssh := &easyssh.MakeConfig{
		User:     user,
		Password: password,
		Port:     "22",
		Server:   ip,
		Key:      "~/.ssh/id_rsa.pub",
	}

	fileName := FileName(src)

	err := ssh.Scp(src, fileName)
	if err != nil {
		return err
	} else {
		_, err := RunSshWithTimeout(ssh, fmt.Sprintf("sudo mv ~/%s %s", fileName, dst), SshExtendedCommandTimeout)
		if err != nil {
			return errors.New("Error moving file: " + err.Error())
		}
	}
	return nil
}

func ScpWithoutSudo(src, dst, ip, user, password string) error {
	ssh := &easyssh.MakeConfig{
		User:     user,
		Password: password,
		Port:     "22",
		Server:   ip,
		Key:      "~/.ssh/id_rsa.pub",
	}

	fileName := FileName(src)
	err := ssh.Scp(src, fileName)
	if err != nil {
		fmt.Println("[-] Error uploading file:", err.Error())
		return err
	} else {
		//tokenize url
		_, err := RunSshWithTimeout(ssh, fmt.Sprintf("mv ~/%s %s", fileName, dst), SshExtendedCommandTimeout)
		if err != nil {
			fmt.Println("[+] Error moving file: ", err.Error())
			return err
		}
	}
	return nil
}

func ScpWithPwd(src, dst, ip, user, password string) error {
	ssh := &easyssh.MakeConfig{
		User:     user,
		Password: password,
		Port:     "22",
		Server:   ip,
		Key:      "~/.ssh/id_rsa.pub",
	}

	fileName := FileName(src)
	err := ssh.Scp(src, fileName)
	if err != nil {
		fmt.Println("[-] Error uploading file:", err.Error())
		return err
	} else {

		fmt.Println("\n[+] Successfully uploaded file : ", fileName)
		_, err := RunSshWithTimeout(ssh, fmt.Sprintf("echo %s | sudo -S mv ~/%s %s", password, fileName, dst), SshExtendedCommandTimeout)
		if err != nil {
			fmt.Println("[+] Error moving file: ", err.Error())
			return err
		}
	}
	return nil
}

func ScpSilentWithPwd(src, dst, ip, user, password string) error {
	ssh := &easyssh.MakeConfig{
		User:     user,
		Password: password,
		Port:     "22",
		Server:   ip,
		Key:      "~/.ssh/id_rsa.pub",
	}
	fileName := FileName(src)

	err := ssh.Scp(src, fileName)
	if err != nil {
		return err
	} else {
		_, err := RunSshWithTimeout(ssh, fmt.Sprintf("echo %s | sudo -S mv ~/%s %s", password, fileName, dst), SshExtendedCommandTimeout)
		if err != nil {
			return errors.New("Error moving file: " + err.Error())
		}
	}
	return nil
}

func EstablishConnection(ips []string, user, password string) string {
	if len(ips) > 1 {
		fmt.Println("[+] More than one IP found")
		for _, ip := range ips {
			if EstablishConn(ip, user, password) {
				return ip
			}
		}
	} else {
		return ips[0]
	}
	return ""
}

func EstablishConn(ip, user, passwd string) bool {
	fmt.Printf("[+] Trying to reach %s@%s\n", user, ip)
	ssh := &easyssh.MakeConfig{
		User:     user,
		Server:   ip,
		Password: passwd,
		Port:     "22",
	}
	resp, err := RunSshWithTimeout(ssh, "whoami", 30)
	if err != nil {
		fmt.Printf("[-] Host is unreachable %s@%s\n", user, ip)
		return false
	} else {
		fmt.Println("[+] Command `whoami` result: ", strings.Trim(resp, "\n"))
		return true
	}
	return false
}

// sudo commands
func RunSshWithTimeout(ssh *easyssh.MakeConfig, command string, timeout int) (string, error) {
	type Result struct {
		Out string
		Err error
	}

	resultChan := make(chan Result)

	go func(resultChan chan Result) {
		out, _, _, err := ssh.Run(fmt.Sprintf("%s", command), SshExtendedCommandTimeout)
		resultChan <- Result{
			Out: out,
			Err: err,
		}
	}(resultChan)

	select {
	case result := <-resultChan:
		return result.Out, result.Err
	case <-time.NewTimer(time.Second * time.Duration(timeout)).C:
		return "", errors.New(fmt.Sprintf("Stopped by timeout %d seconds", timeout))
	}
}

func GenericRunOverSsh(command, ip, user, password, port string, sudo bool, sudoPass string, verbose bool, timeout int) (string, error) {
	ssh := &easyssh.MakeConfig{
		User:     user,
		Password: password,
		Port:     port,
		Server:   ip,
		Key:      "~/.ssh/id_rsa.pub",
	}

	if sudo {
		command = "sudo " + command
	}

	if sudo && sudoPass != "" {
		command = fmt.Sprintf("echo %s | sudo -S %s", sudoPass, command)
	}

	if verbose {
		fmt.Printf("[+] Executing %s %s@%s\n", fmt.Sprintf("sudo %s", command), user, ip)
	}

	out, err := RunSshWithTimeout(ssh, fmt.Sprintf("sudo %s", command), timeout)
	if err != nil {
		fmt.Println("[-] Error running command : ", command, " err msg:", err.Error())
		return out, err
	}
	return out, nil
}

func RunSudoOverSshTimeout(command, ip, user, password string, verbose bool, timeout int) (string, error) {
	return GenericRunOverSsh(command, ip, user, password, DefaultSshPort, true, "", verbose, timeout)
}

func RunSudoOverSsh(command, ip, user, password string, verbose bool) (string, error) {
	return GenericRunOverSsh(command, ip, user, password, DefaultSshPort, true, "", verbose, SshCommandTimeout)
}

func RunOverSsh(command, ip, user, password string) (string, error) {
	return GenericRunOverSsh(command, ip, user, password, DefaultSshPort, false, "", false, SshCommandTimeout)
}

func RunOverSshWithPwd(command, ip, user, password string) (string, error) {
	return GenericRunOverSsh(command, ip, user, password, DefaultSshPort, true, password, false, SshCommandTimeout)
}

func Copy(src, dst string) error {
	sourcefile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourcefile.Close()
	destfile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destfile.Close()
	_, err = io.Copy(destfile, sourcefile)
	if err == nil {
		sourceInfo, err := os.Stat(src)
		if err != nil {
			err = os.Chmod(dst, sourceInfo.Mode())
		}
	}
	return nil
}

func CopyDir(src, dst string) error {
	// get properties of source dir
	sourceinfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	// create dest dir
	err = os.MkdirAll(dst, sourceinfo.Mode())
	if err != nil {
		return err
	}
	directory, _ := os.Open(src)
	objects, err := directory.Readdir(-1)
	for _, obj := range objects {
		srcp := src + "/" + obj.Name()
		dstp := dst + "/" + obj.Name()
		if obj.IsDir() {
			// create sub-directories - recursively
			err = CopyDir(srcp, dstp)
			if err != nil {
				fmt.Println(err)
			}
		} else {
			// perform copy
			err = Copy(srcp, dstp)
			if err != nil {
				fmt.Println(err)
			}
		}
	}
	return nil
}

func CheckRoot() bool {
	user, _ := user.Current()
	if user.Username != "root" {
		fmt.Println("Warning: current user doesn't have root access\nuse \"sudo\"")
		return false
	}

	return true
}

func Abs(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		log.Error("Error getting absolute path ", "err msg:", err.Error())
		return ""
	}
	return abs
}

func GetBinPath() string {
	switch runtime.GOOS {
	case "linux":
		return "/usr/local/bin"
	case "darwin":
		return "/usr/local/bin"
	default:
		return ""
	}
}

// DirExists returns whether the given file or directory exists or not
func DirExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return true, err
}

// return true if file is writable by owner
func FileModeMask(name string, mask os.FileMode) (bool, error) {
	fi, err := os.Stat(name)
	if err != nil {
		return false, err
	}

	return fi.Mode()&mask == mask, nil
}

func InputPassword(data interface{}) string {
	ch, _ := data.(chan bool)

	if ch != nil {
		ch <- false
	}

	fmt.Print("\033[K1\r[+] Enter Password: ")
	pass, _ := gopass.GetPasswdMasked()

	if ch != nil {
		ch <- true
	}

	return string(pass)
}

func CommandExitCode(e error) (int, error) {
	if ee, ok := e.(*exec.ExitError); ok {
		if ws, ok := ee.Sys().(syscall.WaitStatus); ok {
			return ws.ExitStatus(), nil
		}
	}

	return 0, errors.New("Wrong error type")
}

func FileName(path string) string {
	split := strings.Split(path, string(os.PathSeparator))
	name := split[len(split)-1]
	return name
}

func StringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}

	return false
}
