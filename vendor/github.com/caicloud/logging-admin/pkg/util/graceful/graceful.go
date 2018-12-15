package graceful

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/caicloud/nirvana/log"
)

// HandleSignal can catch system signal and send signal to other goroutine before program exits.
// If clear is not empty, it will execute it.
// If waits is not empty, it will wait util all channels in waits being closed.
func HandleSignal(closing chan struct{}, clear func(), waits ...chan struct{}) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	<-sigs
	log.Infoln("capture system signal, will close \"closing\" channel")
	close(closing)
	if clear != nil {
		clear()
	}
	for _, c := range waits {
		<-c
	}
	log.Infoln("exit the process with 0")
	os.Exit(0)
}
