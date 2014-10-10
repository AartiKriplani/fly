package integration_test

import (
	"os/exec"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var flyPath string

var _ = BeforeSuite(func() {
	var err error

	flyPath, err = gexec.Build("github.com/concourse/fly")
	Ω(err).ShouldNot(HaveOccurred())
})

var _ = AfterSuite(gexec.CleanupBuildArtifacts)

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Suite")
}

func tarFiles(path string) string {
	output, err := exec.Command("tar", "tvf", path).Output()
	Expect(err).ToNot(HaveOccurred())

	return string(output)
}
