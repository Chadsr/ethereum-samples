// signs arbitrary information from the command line
package main

import (
	//"encoding/hex"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/user"
	"strings"
	"syscall"

	"golang.org/x/crypto/ssh/terminal"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/common/hexutil"
	//"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	//	"github.com/ethereum/go-ethereum/swarm/storage"
	"github.com/ethereum/go-ethereum/swarm/storage/mru"
)

var (
	g_dir  string
	g_arg  string
	g_file string
	g_help bool
)

func main() {

	usr, err := user.Current()
	if err != nil {
		log.Error("Could not get user info to resolve homedir")
		os.Exit(1)
	}

	var debug bool
	defaultdatadir := fmt.Sprintf("%s/.ethereum", usr.HomeDir)
	flag.StringVar(&g_dir, "d", defaultdatadir, "datadir")
	flag.BoolVar(&g_help, "h", false, "show help")
	flag.BoolVar(&debug, "v", false, "show debug info")
	flag.Usage = func() {
		flag.PrintDefaults()
	}
	flag.Parse()

	if g_help {
		flag.Usage()
		os.Exit(0)
	}

	lvl := log.LvlError
	if debug {
		lvl = log.LvlDebug
	}
	h := log.LvlFilterHandler(lvl, log.StderrHandler)
	log.Root().SetHandler(h)

	if g_dir == "" {
		g_dir = defaultdatadir
	}

	if flag.Arg(0) == "" {
		log.Error("Account or keyfile must be specified")
		os.Exit(1)
	}
	g_arg = flag.Arg(0)

	if flag.Arg(1) == "" {
		log.Error("Specify file to sign")
		os.Exit(1)
	}
	g_file = flag.Arg(1)

	// check if we have file or account
	var keyfile string
	if _, err := hexutil.Decode(g_arg); err != nil {
		log.Debug("input is keyfile")
		fi, err := os.Stat(g_arg)
		if err != nil {
			log.Error("Keyfile not found", "path", g_arg)
			os.Exit(1)
		} else if fi.IsDir() {
			log.Error("Keyfile argument is a directory", "path", g_arg)
			os.Exit(1)
		}
		keyfile = g_arg
	} else {
		log.Debug("input is account hex")
		fi, err := os.Stat(g_dir)
		if err != nil {
			log.Error("Keystore not found", "path", g_dir)
			os.Exit(1)
		} else if !fi.IsDir() {
			log.Error("Keystore is not a directory", "path", g_dir)
			os.Exit(1)
		}

		// search the directory for the key
		keystoredir := fmt.Sprintf("%s/keystore", g_dir)
		log.Debug("checking keystore dir", "dir", keystoredir)
		dircontents, err := ioutil.ReadDir(keystoredir)
		if err != nil {
			log.Error("Can't open keystore dir: %v", err)
		}
		for _, f := range dircontents {
			if strings.Contains(f.Name(), g_arg[2:]) {
				keyfile = fmt.Sprintf("%s/%s", keystoredir, f.Name())
			}
		}
	}

	if keyfile == "" {
		log.Error("Account not found")
		os.Exit(1)
	}

	log.Info("opening account", "keyfile", keyfile)
	j, err := ioutil.ReadFile(keyfile)
	if err != nil {
		log.Error("cannot read file", "err", err)
		os.Exit(1)
	}
	bytePassword := make([]byte, 1024)
	stat, err := os.Stdin.Stat()
	if err != nil && err != io.EOF {
		log.Error("cannot access stdin", "err", err)
		os.Exit(1)
	}
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		total := 0
		for {
			c, err := os.Stdin.Read(bytePassword)
			total += c
			log.Debug("read", "c", c)
			if err != nil {
				if err == io.EOF {
					if bytePassword[total-1] == 0x0a {
						total--
					}
					bytePassword = bytePassword[:total]
					log.Debug("have", "pass", bytePassword)
					break
				}
				fmt.Fprintf(os.Stderr, "read err: %v", err)
				os.Exit(1)
			}
		}
	} else {
		fmt.Printf("pass:")
		bytePassword, err = terminal.ReadPassword(int(syscall.Stdin))
	}
	passphrase := string(bytePassword)
	fmt.Println("\ndecrypting keyfile...")
	key, err := keystore.DecryptKey(j, passphrase)
	if err != nil {
		log.Error("key decrypt failed", "err", err)
		os.Exit(1)
	}

	content, err := ioutil.ReadFile(g_file)
	if err != nil {
		log.Error("read data to sign fail", "err", err)
		os.Exit(1)
	}

	contentBytes, err := hexutil.Decode(string(content))
	if err != nil {
		log.Error("failed to convert content to bytes", "err", err)
		os.Exit(1)
	}

	signer := mru.NewGenericSigner(key.PrivateKey)

	//	hsh := storage.MakeHashFunc(storage.SHA3Hash)()
	//	hsh.Reset()
	//	hsh.Write(contentBytes)

	updateSignature, err := signer.Sign(common.BytesToHash(contentBytes))
	if err != nil {
		log.Error("sign fail", "err", err)
		os.Exit(1)
	}

	fmt.Println(hexutil.Encode(updateSignature[:]))
}
