package main

import (
	//"archive/tar"
	//"compress/gzip"
	"flag"
	"fmt"
	"github.com/op/go-logging"
	"io"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	//"path/filepath"
	"os/exec"
)

var log = logging.MustGetLogger("main")

var archiveFilename = flag.String("archive", "", "The archive to use. Setting this option will surpress the download.")
var boxName = flag.String("name", "", "The name of the box.")

func main() {
	flag.Parse()

	fmt.Println("Welcome to grab-box\n")

	if archiveFilename == nil || *archiveFilename == "" {
		fmt.Println("What is the username/boxname of the box?")

		var boxurl string
		if _, err := fmt.Scanln(&boxurl); err != nil {
			log.Fatal(err)
		}
		fmt.Println("")

		filename, err := downloadBox(boxurl)
		if err != nil {
			log.Fatal(err)
		}

		archiveFilename = &filename
	}

	if boxName == nil || *boxName == "" {
		fmt.Println("What is the name of the box?\n")

		if _, err := fmt.Scanln(boxName); err != nil {
			log.Fatal(err)
		}
	}

	unpackArchive(*archiveFilename, *boxName)

	fmt.Println("\nFixing lxc config...")
	fixConfig(fmt.Sprintf("/var/lib/lxc/%v/config", *boxName), *boxName)

	fmt.Printf("\n\nFinished! You can execute the following command to start the container:\n\n")
	fmt.Printf("\tsudo lxc-start -n '%v'", *boxName)
}

func unpackArchive(filename string, boxname string) {
	containerDir := fmt.Sprintf("/var/lib/lxc/%v", boxname)

	if err := os.MkdirAll(containerDir, 0777); err != nil {
		log.Fatal(err)
	}

	fmt.Println("\nUnarchiving container...")
	untar(filename, containerDir)
}

func fixConfig(filename string, boxName string) {
	config := `# Template used to create this container: ubuntu
# Template script checksum (SHA-1): 6f468a9a658112f6420fb39d2ab90a80fd43cd22

lxc.network.type = veth
lxc.network.hwaddr = 00:16:3e:dd:01:4e
lxc.network.link = lxcbr0
lxc.network.flags = up

lxc.rootfs = /var/lib/lxc/` + boxName + `/rootfs
lxc.mount = /var/lib/lxc/` + boxName + `/fstab
lxc.pivotdir = lxc_putold

lxc.devttydir = lxc
lxc.tty = 4
lxc.pts = 1024

lxc.utsname = ` + boxName + `
lxc.arch = amd64
lxc.cap.drop = sys_module mac_admin mac_override

# When using LXC with apparmor, uncomment the next line to run unconfined:
#lxc.aa_profile = unconfined

lxc.cgroup.devices.deny = a
# Allow any mknod (but not using the node)
lxc.cgroup.devices.allow = c *:* m
lxc.cgroup.devices.allow = b *:* m
# /dev/null and zero
lxc.cgroup.devices.allow = c 1:3 rwm
lxc.cgroup.devices.allow = c 1:5 rwm
# consoles
lxc.cgroup.devices.allow = c 5:1 rwm
lxc.cgroup.devices.allow = c 5:0 rwm
#lxc.cgroup.devices.allow = c 4:0 rwm
#lxc.cgroup.devices.allow = c 4:1 rwm
# /dev/{,u}random
lxc.cgroup.devices.allow = c 1:9 rwm
lxc.cgroup.devices.allow = c 1:8 rwm
lxc.cgroup.devices.allow = c 136:* rwm
lxc.cgroup.devices.allow = c 5:2 rwm
# rtc
lxc.cgroup.devices.allow = c 254:0 rwm
#fuse
lxc.cgroup.devices.allow = c 10:229 rwm
#tun
lxc.cgroup.devices.allow = c 10:200 rwm
#full
lxc.cgroup.devices.allow = c 1:7 rwm
#hpet
lxc.cgroup.devices.allow = c 10:228 rwm
#kvm
lxc.cgroup.devices.allow = c 10:232 rwm
`

	f, err := os.Create(filename)
	if err != nil {
		log.Fatal(err)
	}

	_, err = f.WriteString(config)
	if err != nil {
		log.Fatal(err)
	}
}

// Downloads the box archive to a temporary file. It returns the filename
// to the temporary file, or an error.
func downloadBox(url string) (string, error) {
	fmt.Printf("Downloading box: %v\n", url)

	response, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("Unable to create response for url: %v", err)
	}
	defer response.Body.Close()

	archiveFile, err := ioutil.TempFile(os.TempDir(), "box.tar.gz")
	if err != nil {
		return "", fmt.Errorf("Unable to create temporary file: %v", err)
	}
	defer archiveFile.Close()

	percentageComplete := float64(0)
	bytesWritten := int64(0)
	buffer := make([]byte, 32*1024)
	for {
		nRead, errRead := response.Body.Read(buffer)
		if nRead > 0 {
			nWritten, errWrite := archiveFile.Write(buffer[0:nRead])
			if nWritten > 0 {
				bytesWritten += int64(nWritten)
			}
			if errWrite != nil {
				log.Fatalf("Error writing file: %v", err)
			}
			if nRead != nWritten {
				log.Fatal(io.ErrShortWrite)
			}
		}
		if errRead == io.EOF {
			break
		}
		if errRead != nil {
			log.Fatal("Error ")
		}

		newPercentageComplete := (float64(100) / float64(response.ContentLength)) * float64(bytesWritten)
		if math.Floor(newPercentageComplete) != math.Floor(percentageComplete) {
			fmt.Print(".")
			percentageComplete = newPercentageComplete
		}
	}

	fmt.Print("Complete!\n\n")
	return archiveFile.Name(), nil
}

func untar(filename string, directory string) {
	tarPath, err := exec.LookPath("tar")
	if err != nil {
		log.Fatal(err)
	}

	//cmd := exec.Command(tarPath, fmt.Sprintf("xfz \"%v\" -C \"%v\"", filename, directory))
	cmd := exec.Command(tarPath, "xfz", filename, "-C", directory)
	if err := cmd.Run(); err != nil {
		log.Fatal(err)
	}

	out, _ := cmd.Output()
	log.Debug(string(out))
	// file, err := os.Open(filename)
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// defer file.Close()

	// gzipReader, err := gzip.NewReader(file)
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// reader := tar.NewReader(gzipReader)
	// for {
	// 	header, err := reader.Next()
	// 	if err == io.EOF {
	// 		break
	// 	}

	// 	if err != nil {
	// 		log.Fatal(err)
	// 	}

	// 	if header.FileInfo().IsDir() {
	// 		dir := filepath.Join(directory, header.Name)
	// 		os.Mkdir(dir, header.FileInfo().Mode())
	// 	} else {
	// 		path := filepath.Join(directory, header.Name)
	// 		fileinfo := header.FileInfo()

	// 		file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, fileinfo.Mode()|os.ModeSetuid|os.ModeSetgid)
	// 		if err != nil {
	// 			log.Fatal(err)
	// 		}

	// 		if _, err := io.Copy(file, reader); err != nil {
	// 			log.Fatal(err)
	// 		}

	// 		if err := file.Chown(header.Uid, header.Gid); err != nil {
	// 			log.Fatal(err)
	// 		}
	// 		file.Close()

	// 		if err := os.Chown(path, header.Uid, header.Gid); err != nil {
	// 			log.Fatal(err)
	// 		}
	// 	}
	// }

	fmt.Printf("Container created in: %v", directory)
}
