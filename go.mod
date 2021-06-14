module github.com/shipa-corp/ketch

go 1.16

require (
	bou.ke/monkey v1.0.2
	github.com/BurntSushi/toml v0.3.1
	github.com/buildpacks/pack v0.15.1
	github.com/crossplane/crossplane-runtime v0.13.0
	github.com/go-logr/logr v0.4.0
	github.com/google/go-cmp v0.5.5
	github.com/google/go-containerregistry v0.1.4
	github.com/mattn/go-runewidth v0.0.7 // indirect
	github.com/pkg/errors v0.9.1
	github.com/spf13/cobra v1.1.3
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.0
	github.com/thediveo/enumflag v0.10.1
	gopkg.in/src-d/go-git.v4 v4.13.1
	gopkg.in/yaml.v2 v2.4.0
	gotest.tools v2.2.0+incompatible
	helm.sh/helm/v3 v3.6.0
	k8s.io/api v0.21.1
	k8s.io/apimachinery v0.21.1
	k8s.io/cli-runtime v0.21.1
	k8s.io/client-go v0.21.1
	k8s.io/kubectl v0.21.1
	rsc.io/letsencrypt v0.0.3 // indirect
	sigs.k8s.io/controller-runtime v0.9.0
	sigs.k8s.io/yaml v1.2.0
)

//replace k8s.io/api => k8s.io/api v0.18.8
