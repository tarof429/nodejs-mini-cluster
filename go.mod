module github.com/tarof429/nginx-mini-cluster

go 1.14

require (
	github.com/mitchellh/go-homedir v1.1.0
	github.com/spf13/cobra v1.0.0
	github.com/spf13/viper v1.7.1
	github.com/tarof429/nmc v0.0.0
)

replace github.com/tarof429/nmc => ./nmc
