package flow

import (
	"fmt"
	"trx/internal/config"

	. "github.com/onsi/ginkgo/v2"
	"github.com/werf/trdl/server/pkg/testutil"
)

type testOptions struct {
	trxConfig *config.Config

	mainCfgPath string

	addtasksInMainCfg bool

	pgpKeys map[string]string
	tag1    string
}

var _ = Describe("trx flow test", Label("e2e", "trx", "flow"), func() {
	DescribeTable("should perform all steps",
		func(testOpts testOptions) {
			By("initializing git repo")
			{

				initGitRepo(SuiteData.TestDir, "main")

			}
			By("creating main config file")
			{
				testOpts.trxConfig = &config.Config{
					Repo: &config.GitRepo{
						Url: SuiteData.TestDir,
					},
					Quorums: []config.Quorum{
						{
							Name:            func(s string) *string { return &s }("dev"),
							MinNumberOfKeys: 1,
							GPGKeys: []string{
								keyMap["developer"],
							},
							GPGKeyFilesPaths: nil,
						},
						{
							Name:             func(s string) *string { return &s }("prod"),
							MinNumberOfKeys:  1,
							GPGKeyFilesPaths: []string{FixturePath("pgp_keys", "pm_public.pgp")},
						},
					},
					Hooks: config.Hooks{
						Env: map[string]string{
							"WERF_SET_GIT_REV": "werf.git_rev={{ .RepoCommit }}",
							"MESSAGE":          `:eight_pointed_black_star: Start converge {{ .RepoTag }} for ({{ .RepoUrl }})`,
						},
						OnCommandStarted: &[]string{
							"echo 'task !!{{ .StartedTaskName }}!! started'", "echo $WERF_SET_GIT_REV",
						},
						OnCommandSuccess: &[]string{
							"echo 'command success'",
						},
						OnCommandFailure: &[]string{
							"echo 'command failure'",
						},
						OnCommandSkipped: &[]string{
							"echo $MESSAGE",
						},
					},
				}

				if testOpts.addtasksInMainCfg {
					testOpts.trxConfig.Tasks = []config.Task{
						{
							Name:                    "dev",
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
					}
				}

				testOpts.mainCfgPath, _ = writeConfigFile(SuiteData.TestDir, testOpts.trxConfig)

			}
			By(fmt.Sprintf("Creating tag tag %q", testOpts.tag1))
			{
				gitTag(SuiteData.TestDir, testOpts.tag1, testOpts.pgpKeys["developer"])

				By(fmt.Sprintf("[server] Signing tag %q", testOpts.tag1))
				quorumSignTag(SuiteData.TestDir, testOpts.pgpKeys["tl"], testOpts.pgpKeys["pm"], testOpts.tag1)
			}
			By(fmt.Sprintf("Running trx with tag %q", testOpts.tag1))
			{
				testutil.RunSucceedCommand(
					"",
					SuiteData.TrxBinPath,
					"--config", testOpts.mainCfgPath,
				)
			}
			By(fmt.Sprintf("Running trx with tag %q should be skipped", testOpts.tag1))
			{
				testutil.RunSucceedCommand(
					"",
					SuiteData.TrxBinPath,
					"--config", testOpts.mainCfgPath,
				)
			}
			By(fmt.Sprintf("Running trx with tag %q another task should successed", testOpts.tag1))
			{
				testutil.RunSucceedCommand(
					"",
					SuiteData.TrxBinPath,
					"--config", testOpts.mainCfgPath, "--task", "test",
				)
			}
			By(fmt.Sprintf("Running trx with tag %q force", testOpts.tag1))
			{
				testutil.RunSucceedCommand(
					"",
					SuiteData.TrxBinPath,
					"--config", testOpts.mainCfgPath, "--task", "test", "--force",
				)
			}
			By(fmt.Sprintf("Running trx with tag %q force cli", testOpts.tag1))
			{
				testutil.RunSucceedCommand(
					"",
					SuiteData.TrxBinPath,
					"--config", testOpts.mainCfgPath, "--force", "--", "ls",
				)
			}
		},
		Entry("standart test -- tasks from main file", testOptions{
			tag1: "v0.0.1",
			pgpKeys: map[string]string{
				"developer": "74E1259029B147CB4033E8B80D4C9C140E8A1030",
				"tl":        "2BA55FD8158034EEBE92AA9ED9D79B63AFC30C7A",
				"pm":        "C353F279F552B3EF16DAE0A64354E51BF178F735",
			},
			addtasksInMainCfg: true,
		}),
		Entry("standart test -- tasks from repo file", testOptions{
			tag1: "v0.0.1",
			pgpKeys: map[string]string{
				"developer": "74E1259029B147CB4033E8B80D4C9C140E8A1030",
				"tl":        "2BA55FD8158034EEBE92AA9ED9D79B63AFC30C7A",
				"pm":        "C353F279F552B3EF16DAE0A64354E51BF178F735",
			},
			addtasksInMainCfg: false,
		}),
	)
})

var keyMap = map[string]string{
	"developer": `-----BEGIN PGP PUBLIC KEY BLOCK-----

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
XaOlJrPDM5E9zw==
=bIYD
-----END PGP PUBLIC KEY BLOCK-----
	`,
}
