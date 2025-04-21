package flow

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"trx/internal/config"

	. "github.com/onsi/gomega"
	"github.com/werf/trdl/server/pkg/testutil"
	"gopkg.in/yaml.v3"
)

func FixturePath(paths ...string) string {
	absFixturesPath, err := filepath.Abs("_fixtures")
	Ω(err).ShouldNot(HaveOccurred())
	pathsToJoin := append([]string{absFixturesPath}, paths...)
	return filepath.Join(pathsToJoin...)
}

func importGPGKeys(keys map[string]string) {
	for user := range keys {
		testutil.RunSucceedCommand(
			testutil.FixturePath("pgp_keys"),
			"gpg",
			"--import",
			fmt.Sprintf("%s_private.pgp", user),
		)
	}
}

func removeGPGKeys(keys []string) {
	for _, keyId := range keys {
		testutil.RunSucceedCommand(
			testutil.FixturePath("pgp_keys"),
			"gpg",
			"--batch", "--yes", "--delete-secret-and-public-key",
			keyId,
		)
	}
}

func initGitRepo(testDir, branchName string) {
	testutil.CopyIn(testutil.FixturePath("complete_cycle"), testDir)

	testutil.RunSucceedCommand(
		testDir,
		"git",
		"-c", "init.defaultBranch="+branchName,
		"init",
	)

	testutil.RunSucceedCommand(
		testDir,
		"touch", "testfile",
	)

	writeRunnerConfigFile(testDir, &config.RunnerConfig{
		Tasks: []config.Task{
			{
				Name:                    "dev-repo",
				Env:                     map[string]string{"TRX_ENV": "dev"},
				Commands:                []string{"echo 'hello world'", "echo $TRX_ENV"},
				InitialLastProcessedTag: "",
			},
			{
				Name:                    "test",
				Env:                     map[string]string{"TRX_ENV": "test"},
				Commands:                []string{"echo 'hello world'", "echo $TRX_ENV"},
				InitialLastProcessedTag: "",
			},
		},
	})

	testutil.RunSucceedCommand(
		testDir,
		"git",
		"add", "-A",
	)

	testutil.RunSucceedCommand(
		testDir,
		"git",
		"commit", "-m", "Initial commit",
	)
}

func gitTag(testDir, tag, pgpSigningKeyDeveloper string) {
	testutil.RunSucceedCommand(
		testDir,
		"git",
		"-c", "tag.gpgsign=true",
		"-c", "user.signingkey="+pgpSigningKeyDeveloper,
		"tag", tag, "-m", "New version",
	)
}

func quorumSignTag(testDir, pgpSigningKeyTL, pgpSigningKeyPM, tag string) {
	if runtime.GOOS == "darwin" {
		err := os.Setenv("GIT_EDITOR", `vim -c ":normal iNew version" -c ":wq"`)
		Ω(err).ShouldNot(HaveOccurred())
	}
	testutil.RunSucceedCommand(
		testDir,
		"git",
		"signatures", "add", "--key", pgpSigningKeyTL, tag,
	)

	testutil.RunSucceedCommand(
		testDir,
		"git",
		"signatures", "add", "--key", pgpSigningKeyPM, tag,
	)
}

func writeRunnerConfigFile(path string, cfg *config.RunnerConfig) (string, error) {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("failed to marshal config: %w", err)
	}

	filePath := filepath.Join(path, "trx.yaml")
	err = os.WriteFile(filePath, data, 0644)
	if err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	testutil.RunSucceedCommand(
		"",
		"cat", filePath,
	)

	return filePath, nil
}

func writeConfigFile(path string, cfg *config.Config) (string, error) {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("failed to marshal config: %w", err)
	}

	filePath := filepath.Join(path, "trx.yaml")
	err = os.WriteFile(filePath, data, 0644)
	if err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	testutil.RunSucceedCommand(
		"",
		"cat", filePath,
	)

	return filePath, nil
}
