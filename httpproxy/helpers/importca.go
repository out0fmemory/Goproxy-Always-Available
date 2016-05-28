package helpers

import (
	"os/exec"
	"runtime"
)

func ImportCAToSystemRoot(name, filename string) error {
	cmds := make([]*exec.Cmd, 0)
	switch runtime.GOOS {
	case "windows":
		cmds = append(cmds, exec.Command("certmgr.exe", "-del", "-c", "-n", name, "-s", "-r", "localMachine", "root"))
		cmds = append(cmds, exec.Command("certmgr.exe", "-add", "-c", filename, "-s", "-r", "localMachine", "root"))
	case "darwin":
		cmds = append(cmds, exec.Command("security", "add-trusted-cert", "-d", "-r", "trustRoot", "-k", "/Library/Keychains/System.keychain", filename))
	default:
		break
	}

	for i, cmd := range cmds {
		err := cmd.Run()
		if err != nil && i == len(cmds)-1 {
			return err
		}
	}
	return nil
}
