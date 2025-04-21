package flow

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/prashantv/gostub"
	"github.com/werf/trdl/server/pkg/testutil"
)

func Test(t *testing.T) {
	testutil.MeetsRequirementTools([]string{"git", "git-signatures", "gpg"})
	RegisterFailHandler(Fail)
	RunSpecs(t, "Flow Suite")
}

var SuiteData = struct {
	TrxBinPath string

	TmpDir  string
	TestDir string

	Stubs *gostub.Stubs

	GPGKeys []string
}{}
var (
	_ = BeforeSuite(func() {
		SuiteData.TrxBinPath = BuildTrxBin()
		keys := map[string]string{
			"developer": "74E1259029B147CB4033E8B80D4C9C140E8A1030",
			"tl":        "2BA55FD8158034EEBE92AA9ED9D79B63AFC30C7A",
			"pm":        "C353F279F552B3EF16DAE0A64354E51BF178F735",
		}
		importGPGKeys(keys)
		for _, v := range keys {
			SuiteData.GPGKeys = append(SuiteData.GPGKeys, v)
		}
	})

	_ = BeforeEach(func() {
		SuiteData.Stubs = gostub.New()
		SuiteData.TmpDir = testutil.GetTempDir()

		SuiteData.TestDir = filepath.Join(SuiteData.TmpDir, "trx-project")
		Ω(os.Mkdir(SuiteData.TestDir, os.ModePerm))
	})

	_ = AfterEach(func() {
		err := os.RemoveAll(SuiteData.TmpDir)
		Ω(err).ShouldNot(HaveOccurred())
	})

	_ = AfterSuite(func() {
		removeGPGKeys(SuiteData.GPGKeys)
	})
)

func BuildTrxBin() string {
	testutil.RunSucceedCommand(
		"",
		"task",
		"build",
		"-d", "../../",
	)
	return "../../bin/trx"
}
