package immuclient

import (
	c "github.com/codenotary/immudb/cmd/helper"
	"github.com/codenotary/immudb/cmd/immuclient/immuc"
	"github.com/codenotary/immudb/pkg/client"
	"github.com/spf13/cobra"
)

type commandline struct {
	immucl  immuc.Client
	config  c.Config
	onError func(msg interface{})
	options *client.Options
}

func NewCommandLine() commandline {
	cl := commandline{}
	cl.config.Name = "immuclient"
	cl.options = client.DefaultOptions()
	return cl
}

func (cl *commandline) ConfigChain(post func(cmd *cobra.Command, args []string) error) func(cmd *cobra.Command, args []string) (err error) {
	return func(cmd *cobra.Command, args []string) (err error) {
		if err = cl.config.LoadConfig(cmd); err != nil {
			return err
		}
		cl.options = immuc.Options()
		cl.immucl, err = immuc.Init(cl.options)
		if post != nil {
			return post(cmd, args)
		}
		return nil
	}
}

// Register ...
func (cl *commandline) Register(rootCmd *cobra.Command) *cobra.Command {
	// login and logout
	cl.login(rootCmd)
	cl.logout(rootCmd)
	// current status
	cl.currentRoot(rootCmd)
	// get operations
	cl.getByIndex(rootCmd)
	cl.getRawBySafeIndex(rootCmd)
	cl.getKey(rootCmd)
	cl.rawSafeGetKey(rootCmd)
	cl.safeGetKey(rootCmd)
	// set operations
	cl.rawSafeSet(rootCmd)
	cl.set(rootCmd)
	cl.safeset(rootCmd)
	cl.zAdd(rootCmd)
	cl.safeZAdd(rootCmd)
	// scanners
	cl.zScan(rootCmd)
	cl.iScan(rootCmd)
	cl.scan(rootCmd)
	cl.count(rootCmd)
	// references
	cl.reference(rootCmd)
	cl.safereference(rootCmd)
	// misc
	cl.inclusion(rootCmd)
	cl.consistency(rootCmd)
	cl.history(rootCmd)
	cl.status(rootCmd)
	cl.auditmode(rootCmd)
	cl.interactiveCli(rootCmd)
	cl.use(rootCmd)
	return rootCmd
}

func (cl *commandline) connect(cmd *cobra.Command, args []string) (err error) {
	err = cl.immucl.Connect(args)
	if err != nil {
		cl.quit(err)
	}
	return
}

func (cl *commandline) disconnect(cmd *cobra.Command, args []string) {
	if err := cl.immucl.Disconnect(args); err != nil {
		cl.quit(err)
	}
}

func (cl *commandline) quit(msg interface{}) {
	if cl.onError == nil {
		c.QuitToStdErr(msg)
	}
	cl.onError(msg)
}