package helpers

import (
	"os/exec"
)

func ImportCAToSystemRoot(name, filename string) error {
	cmds := make([]*exec.Cmd, 0)
	cmds = append(cmds, exec.Command("certmgr.exe", "-del", "-c", "-n", name, "-s", "-r", "localMachine", "root"))
	cmds = append(cmds, exec.Command("certmgr.exe", "-add", "-c", filename, "-s", "-r", "localMachine", "root"))

	for i, cmd := range cmds {
		err := cmd.Run()
		if err != nil && i == len(cmds)-1 {
			return err
		}
	}
	return nil
}
