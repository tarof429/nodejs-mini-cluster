package nmc

import "testing"

func TestMain(t *testing.T) {

	for i := 0; i < 10; i++ {
		port := GetRoundRobinForwardPort()

		if port != "3001" && port != "3002" {
			t.Fail()
		}

	}

}
