package main

import localmanager "configmap-controller/manager"

func main() {
	err := localmanager.RunManager()
	if err != nil {
		panic(err)
	}
}
