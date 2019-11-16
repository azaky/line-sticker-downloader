package util

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
)

func Download(filepath string, url string) error {
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status code: %s", resp.Status)
	}

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	return nil
}

func execCmd(cmd *exec.Cmd) (error, string) {
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err, ""
	}
	err = cmd.Start()
	if err != nil {
		log.Fatalf("Error running %s: %s", cmd.Path, err.Error())
		return err, ""
	}
	output, err := ioutil.ReadAll(stderr)
	if err != nil {
		log.Fatalf("Error reading stderr for command %s: %s", cmd.Path, err.Error())
		return err, ""
	}
	err = cmd.Wait()
	if err != nil {
		log.Printf("Error running %s: %s", cmd.Path, err.Error())
	}
	return err, string(output)
}

func ExecDir(dir, name string, args ...string) (error, string) {
	log.Printf("Exec [%s] with dir [%s] with args %v", name, dir, args)
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	return execCmd(cmd)
}

func Exec(name string, args ...string) (error, string) {
	log.Printf("Exec [%s] with args %v", name, args)
	cmd := exec.Command(name, args...)
	return execCmd(cmd)
}
