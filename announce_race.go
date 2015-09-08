// +build race

package main

func announce() {
	log.Warn(`You have compiled wago with the go race detector (-race).
   There are a number of existing race conditions,
   all relating to os/exec.Wait() and killing processes.
   These are harmless to regular usage, for details see:`)
	log.Warn(`https://github.com/JonahBraun/wago/issues/1`)
}
