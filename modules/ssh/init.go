// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package ssh

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

func Init() error {
	if setting.SSH.Disabled {
		builtinUnused()
		return nil
	}

	if setting.SSH.StartBuiltinServer {
		if len(setting.SSH.ListenHost) > 1 {
			graceful.GetManager().IncreaseListenerCountBy(len(setting.SSH.ListenHost) - 1)
		}
		for _, listenHost := range setting.SSH.ListenHost {
			Listen(listenHost, setting.SSH.ListenPort, setting.SSH.ServerCiphers, setting.SSH.ServerKeyExchanges, setting.SSH.ServerMACs)
			log.Info("SSH server started on %s. Cipher list (%v), key exchange algorithms (%v), MACs (%v)",
				net.JoinHostPort(listenHost, strconv.Itoa(setting.SSH.ListenPort)),
				setting.SSH.ServerCiphers, setting.SSH.ServerKeyExchanges, setting.SSH.ServerMACs,
			)
		}
		return nil
	}

	builtinUnused()

	// FIXME: why 0o644 for a directory .....
	if err := os.MkdirAll(setting.SSH.KeyTestPath, 0o644); err != nil {
		return fmt.Errorf("failed to create directory %q for ssh key test: %w", setting.SSH.KeyTestPath, err)
	}

	if len(setting.SSH.TrustedUserCAKeys) > 0 && setting.SSH.AuthorizedPrincipalsEnabled {
		caKeysFileName := setting.SSH.TrustedUserCAKeysFile
		caKeysFileDir := filepath.Dir(caKeysFileName)

		err := os.MkdirAll(caKeysFileDir, 0o700) // SSH.RootPath by default (That is `~/.ssh` in most cases)
		if err != nil {
			return fmt.Errorf("failed to create directory %q for ssh trusted ca keys: %w", caKeysFileDir, err)
		}

		if err := os.WriteFile(caKeysFileName, []byte(strings.Join(setting.SSH.TrustedUserCAKeys, "\n")), 0o600); err != nil {
			return fmt.Errorf("failed to write ssh trusted ca keys to %q: %w", caKeysFileName, err)
		}
	}

	return nil
}
