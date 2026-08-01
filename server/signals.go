package server

import "github.com/kirill-jjj/nvdaRemoteServer/signals"

func signals_init() {
	sig := <-signals.Wait()
	Log(LOG_INFO, "Signal received to shut down. Received signal "+sig.String())
	msl.Lock()
	stoppingServers = true
	StopServers()
	msl.Unlock()
}
