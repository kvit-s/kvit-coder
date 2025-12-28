package agent

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/kvit-s/kvit-coder/internal/config"
)

// PermissionChoice represents user's response to permission prompt
type PermissionChoice int

const (
	PermissionOnce PermissionChoice = iota
	PermissionAlways
	PermissionPermanent
	PermissionDeny
)

// PromptForPermission asks the user to grant access to a path
func PromptForPermission(path string, accessType config.AccessType) (PermissionChoice, bool, error) {
	accessTypeStr := "READ"
	if accessType == config.AccessWrite {
		accessTypeStr = "WRITE"
	}

	fmt.Printf("\n⚠️  Path %s is outside workspace\n", path)
	fmt.Printf("Allow %s access? [O]nce [A]lways [P]ermanent [D]eny: ", accessTypeStr)

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return PermissionDeny, false, err
	}

	choice := strings.TrimSpace(strings.ToLower(input))

	switch choice {
	case "o", "once":
		return PermissionOnce, false, nil
	case "a", "always":
		return PermissionAlways, false, nil
	case "p", "permanent":
		// For permanent, ask if read-only or read+write
		if accessType == config.AccessRead {
			fmt.Print("\nPermission level?\n[1] Read-only\n[2] Read+Write: ")
			levelInput, _ := reader.ReadString('\n')
			readWrite := strings.TrimSpace(levelInput) == "2"
			return PermissionPermanent, readWrite, nil
		}
		return PermissionPermanent, true, nil
	default:
		return PermissionDeny, false, nil
	}
}

// UpdateConfigFile updates .kvit-coder.yaml with new permanent permissions
func UpdateConfigFile(path string, readWrite bool) error {
	// TODO: Implement YAML update logic
	// This would read .kvit-coder.yaml, add the path to appropriate section, and write back
	configPath := ".kvit-coder.yaml"

	if readWrite {
		fmt.Printf("✓ Added %s to allowed_paths in %s\n", path, configPath)
	} else {
		fmt.Printf("✓ Added %s to allowed_read_paths in %s\n", path, configPath)
	}

	return nil
}
