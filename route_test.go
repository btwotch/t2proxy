package main

import (
	"testing"
)

func TestMerge(t *testing.T) {
	devs := []string{"eth0", "wlan0", "tun0", "tap0", "wlan1"}
	preferredDevs := []string{"wlan5", "eth0", "tun0", "wlan0"}
	expectedDevs := []string{"eth0", "tun0", "wlan0"}

	newDevs := mergeDevs(devs, preferredDevs)

	if len(newDevs) != len(expectedDevs) {
		t.Fatalf("newDevs (%+v) != expectedDevs (%+v)", newDevs, expectedDevs)
	}

	for i, _ := range newDevs {
		if newDevs[i] != expectedDevs[i] {
			t.Fatalf("newDevs (%+v) != expectedDevs (%+v)", newDevs, expectedDevs)
		}
	}
}

func TestAllMerge(t *testing.T) {
	devs := []string{"eth0", "wlan0", "tun0", "tap0", "wlan1"}
	preferredDevs := []string{"wlan5", "eth0", "*", "tun0", "wlan0"}
	expectedDevs1 := []string{"eth0", "tap0", "wlan1", "tun0", "wlan0"}
	expectedDevs2 := []string{"eth0", "wlan1", "tap0", "tun0", "wlan0"}

	newDevs := mergeDevs(devs, preferredDevs)

	if len(newDevs) != len(expectedDevs1) {
		t.Fatalf("newDevs (%+v) != expectedDevs1 (%+v) or expectedDevs2 (%+v)", newDevs, expectedDevs1, expectedDevs2)
	}

	success1 := true
	success2 := true

	for i, _ := range newDevs {
		if newDevs[i] != expectedDevs1[i] {
			success1 = false
		}
		if newDevs[i] != expectedDevs2[i] {
			success2 = false
		}
	}

	if !success1 && !success2 {
		t.Fatalf("newDevs (%+v) != expectedDevs1 (%+v) or expectedDevs2 (%+v)", newDevs, expectedDevs1, expectedDevs2)
	}
}
