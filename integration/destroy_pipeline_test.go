package integration_test

import (
	"fmt"
	"io"
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Fly CLI", func() {
	var (
		atcServer *ghttp.Server
	)

	Describe("destroy-pipeline", func() {
		BeforeEach(func() {
			atcServer = ghttp.NewServer()
		})

		Context("when a pipeline name is not specified", func() {
			It("asks the user to specifiy a pipeline name", func() {
				flyCmd := exec.Command(flyPath, "-t", atcServer.URL(), "destroy-pipeline")

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(1))

				Expect(sess.Err).To(gbytes.Say("error: the required flag `" + osFlag("p", "pipeline") + "' was not specified"))
			})
		})

		Context("when a pipeline name is specified", func() {
			var (
				stdin io.Writer
				sess  *gexec.Session
			)

			JustBeforeEach(func() {
				var err error

				flyCmd := exec.Command(flyPath, "-t", atcServer.URL(), "destroy-pipeline", "-p", "some-pipeline")
				stdin, err = flyCmd.StdinPipe()
				Expect(err).NotTo(HaveOccurred())

				sess, err = gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(sess).Should(gbytes.Say("!!! this will remove all data for pipeline `some-pipeline`"))
				Eventually(sess).Should(gbytes.Say(`are you sure\? \(y\/n\): `))
			})

			It("exits successfully if the user confirms", func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("DELETE", "/api/v1/pipelines/some-pipeline"),
						ghttp.RespondWith(204, ""),
					),
				)

				fmt.Fprintln(stdin, "y")
				Eventually(sess).Should(gexec.Exit(0))
			})

			It("bails out if the user presses no", func() {
				fmt.Fprintln(stdin, "n")

				Eventually(sess.Err).Should(gbytes.Say(`bailing out`))
				Eventually(sess).Should(gexec.Exit(1))
			})

			Context("and the pipeline exists", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("DELETE", "/api/v1/pipelines/some-pipeline"),
							ghttp.RespondWith(204, ""),
						),
					)
				})

				It("writes a success message to stdout", func() {
					fmt.Fprintln(stdin, "y")
					Eventually(sess).Should(gbytes.Say("`some-pipeline` deleted"))
					Eventually(sess).Should(gexec.Exit(0))
				})
			})

			Context("and the pipeline does not exist", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("DELETE", "/api/v1/pipelines/some-pipeline"),
							ghttp.RespondWith(404, ""),
						),
					)
				})

				It("writes an error message to stderr", func() {
					fmt.Fprintln(stdin, "y")
					Eventually(sess.Err).Should(gbytes.Say("`some-pipeline` does not exist"))
					Eventually(sess).Should(gexec.Exit(1))
				})
			})

			Context("and the api returns an unexpected status code", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("DELETE", "/api/v1/pipelines/some-pipeline"),
							ghttp.RespondWith(402, ""),
						),
					)
				})

				It("writes an error message to stderr", func() {
					fmt.Fprintln(stdin, "y")
					Eventually(sess.Err).Should(gbytes.Say("Unexpected Response"))
					Eventually(sess).Should(gexec.Exit(1))
				})
			})
		})
	})
})
