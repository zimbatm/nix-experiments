package nixserve

import (
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"regexp"

	 "github.com/zimbatm/nix-store-lan/resource"
)

func queryPathFromHashPart(hashPart string) string {
	// XXX
	return ""
}

func queryPathInfo(hash string) (*PathInfo, error) {
	return nil, nil
}

func stripPath(nixPath string) string {
	return ""
}

type PathInfo struct {
	 deriver string
	 narHash string
	 time string
	 narSize string
	 refs []string
 }

type NixServe struct {
	*http.ServeMux
}

func NewAPI() http.Handler {
	mux := http.NewServeMux()

	s := &NixServe{
		ServeMux: mux,
	}

	mux.Handle("/nix-cache-info", resource.GET(func (w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")

		// TODO: make that configurable
		storeDir := "/nix/store"

		body := fmt.Sprintf("StoreDir: %s\nWantMassQuery: 1\nPriority: 30\n", storeDir)

		w.Write([]byte(body))
	}))

	// TODO: add .nar.bz2 support
	reNarPath := regexp.MustCompile("^/nar/([0-9a-z]+)\\.nar$")
	mux.Handle("/nar/", resource.GET(func (w http.ResponseWriter, r *http.Request) {
		matches := reNarPath.FindStringSubmatch(r.URL.Path)
		if len(matches) == 0 {
			http.Error(w, "invalid nar path", http.StatusNotFound)
			return
		}
		hashPart := matches[0]
		storePath := queryPathFromHashPart(hashPart)
		if storePath == "" {
			http.NotFound(w, r)
			return
		}

		cmd := exec.CommandContext(r.Context(), "nix-store", "--dump", storePath)
		cmd.Stdin = nil
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			w.Header().Set("Content-Type", "text/plain")
			http.Error(w, err.Error(), 500)
			return
		}
		if err := cmd.Start(); err != nil {
			w.Header().Set("Content-Type", "text/plain")
			http.Error(w, err.Error(), 500)
			return
		}

		w.Header().Set("Content-Type", "application/x-nix-nar")

		io.Copy(w, stdout)
	}))

	reNarInfoPath := regexp.MustCompile("^/([0-9a-z]+)\\.narinfo$")
	mux.Handle("/", resource.GET(func (w http.ResponseWriter, r *http.Request) {
		matches := reNarInfoPath.FindStringSubmatch(r.URL.Path)
		if len(matches) == 0 {
			http.Error(w, "invalid narinfo path", http.StatusNotFound)
			return
		}
		hashPart := matches[0]
		storePath := queryPathFromHashPart(hashPart)
		if storePath == "" {
			http.NotFound(w, r)
			return
		}

		pathInfo, err := queryPathInfo(storePath)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		res :=
			"StorePath: $storePath\n" +
			"URL: nar/$hashPart.nar\n" +
			"Compression: none\n" +
			"NarHash: $narHash\n" +
			"NarSize: $narSize\n";

		if len(pathInfo.refs) > 0 {
			res += "References:"
			for _, ref := range pathInfo.refs {
				res += " " + stripPath(ref)
			}
			res += "\n"
		}

		/*
		my $secretKeyFile = $ENV{'NIX_SECRET_KEY_FILE'};
        if (defined $secretKeyFile) {
            my $secretKey = readFile $secretKeyFile;
            chomp $secretKey;
            my $fingerprint = fingerprintPath($storePath, $narHash, $narSize, $refs);
            my $sig = signString($secretKey, $fingerprint);
            $res .= "Sig: $sig\n";
		*/
		w.Header().Set("Content-Type", "text/x-nix-narinfo")
		w.Write([]byte(res))
	}))



	return s
}
