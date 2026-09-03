package main

import (
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// Config
var (
	baseURL = "https://fapi.asterdex.com"
)

func main() {
	userAddr := flag.String("user", "", "Aster Main User Address (0x...)")
	privKey := flag.String("key", "", "Aster Agent Private Key (without 0x)")
	flag.Parse()

	if *userAddr == "" || *privKey == "" {
		fmt.Println("Usage: go run aster_check.go -user <0xUser> -key <PrivateKey>")
		return
	}

	fmt.Printf("🔍 Starting Diagnostics for User: %s\n", *userAddr)

	// 1. Derive Signer from Private Key
	pk, err := crypto.HexToECDSA(strings.TrimPrefix(*privKey, "0x"))
	if err != nil {
		fmt.Printf("❌ Invalid Private Key: %v\n", err)
		return
	}

	pubKey := pk.Public()
	publicKeyECDSA, ok := pubKey.(*ecdsa.PublicKey)
	if !ok {
		fmt.Println("❌ Error casting public key to ECDSA")
		return
	}
	signerAddr := crypto.PubkeyToAddress(*publicKeyECDSA).Hex()
	fmt.Printf("🔑 Derived Signer Address (Agent): %s\n", signerAddr)

	// 2. Check Time
	fmt.Printf("🕒 System Time: %s (UnixMicro: %d)\n", time.Now().Format(time.RFC3339), time.Now().UnixMicro())

	// 3. Test Connectivity (Exchange Info)
	fmt.Println("\n📡 Testing Public Endpoint (Exchange Info)...")
	_, err = http.Get(baseURL + "/fapi/v3/exchangeInfo")
	if err != nil {
		fmt.Printf("❌ Network Error connecting to %s: %v\n", baseURL, err)
		return
	}
	fmt.Println("✅ Public Endpoint Executed Successfully")

	// 4. Test Authenticated Request (Balance)
	fmt.Println("\n🔐 Testing Authenticated Request (Balance)...")
	err = getBalance(*userAddr, signerAddr, pk)
	if err != nil {
		fmt.Printf("❌ Authenticated Request Failed: %v\n", err)
		fmt.Println("\n💡 Troubleshooting Tips:")
		fmt.Println("1. 'No agent found': The Main User has NOT authorized this Agent address.")
		fmt.Println("   -> Go to AsterDex -> Manage Agents -> Add Agent: " + signerAddr)
		fmt.Println("2. 'Invalid signature': Key issue or nonce issue.")
	} else {
		fmt.Println("✅ Authenticated Request Successful! Credentials are valid.")
	}
}

func getBalance(user, signer string, privateKey *ecdsa.PrivateKey) error {
	endpoint := "/fapi/v3/balance"
	params := make(map[string]interface{})

	// Prepare params
	nonce := uint64(time.Now().UnixMicro())
	params["recvWindow"] = "50000"
	params["timestamp"] = strconv.FormatInt(time.Now().UnixNano()/int64(time.Millisecond), 10)

	// 1. Normalize/Stringify for signing
	jsonStr, err := normalizeAndStringify(params)
	if err != nil {
		return fmt.Errorf("normalize failed: %v", err)
	}

	// 2. Pack ABI
	addrUser := common.HexToAddress(user)
	addrSigner := common.HexToAddress(signer)
	nonceBig := new(big.Int).SetUint64(nonce)

	tString, _ := abi.NewType("string", "", nil)
	tAddress, _ := abi.NewType("address", "", nil)
	tUint256, _ := abi.NewType("uint256", "", nil)

	arguments := abi.Arguments{
		{Type: tString},
		{Type: tAddress},
		{Type: tAddress},
		{Type: tUint256},
	}

	packed, err := arguments.Pack(jsonStr, addrUser, addrSigner, nonceBig)
	if err != nil {
		return fmt.Errorf("pack failed: %v", err)
	}

	// 3. Hash & Sign
	hash := crypto.Keccak256(packed)
	prefixedMsg := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(hash), hash)
	msgHash := crypto.Keccak256Hash([]byte(prefixedMsg))

	sig, err := crypto.Sign(msgHash.Bytes(), privateKey)
	if err != nil {
		return fmt.Errorf("sign failed: %v", err)
	}

	// Adjust V
	if len(sig) != 65 {
		return fmt.Errorf("sig len wrong")
	}
	sig[64] += 27 // Transform V from 0/1 to 27/28

	// 4. Add Auth Params
	params["user"] = user
	params["signer"] = signer
	params["signature"] = "0x" + hex.EncodeToString(sig)
	params["nonce"] = nonce

	// 5. Send Request
	return doRequest("GET", endpoint, params)
}

func doRequest(method, endpoint string, params map[string]interface{}) error {
	fullURL := baseURL + endpoint

	q := url.Values{}
	for k, v := range params {
		q.Set(k, fmt.Sprintf("%v", v))
	}

	reqURL := fullURL
	if method == "GET" {
		reqURL = fullURL + "?" + q.Encode()
	}

	req, err := http.NewRequest(method, reqURL, nil)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("📥 Response Status: %s\n", resp.Status)
	fmt.Printf("📥 Response Body: %s\n", string(body))

	if resp.StatusCode != 200 {
		return fmt.Errorf("API Error: %s", string(body))
	}
	return nil
}

// Helpers from original code
func normalizeAndStringify(params map[string]interface{}) (string, error) {
	normalized, err := normalize(params)
	if err != nil {
		return "", err
	}
	bs, err := json.Marshal(normalized)
	if err != nil {
		return "", err
	}
	return string(bs), nil
}

func normalize(v interface{}) (interface{}, error) {
	switch val := v.(type) {
	case map[string]interface{}:
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		newMap := make(map[string]interface{}, len(keys))
		for _, k := range keys {
			nv, err := normalize(val[k])
			if err != nil {
				return nil, err
			}
			newMap[k] = nv
		}
		return newMap, nil
	default:
		return fmt.Sprintf("%v", val), nil
	}
}
