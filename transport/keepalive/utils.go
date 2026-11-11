package keepalive

import (
	"math/rand"

	"github.com/liujitcn/kratos-kit/transport"
)

func generatePort() int {
	return generateNumber(10000, 65535)
}

func generateNumber(minNum, maxNum int) int {
	return rand.Intn(maxNum-minNum) + minNum
}

func generateHost(interfaceName, explicitHost string) (string, error) {
	if interfaceName != "" {
		h, err := transport.GetIPAddressByInterfaceName(interfaceName)
		if err != nil {
			return "", err
		}
		return h, nil
	} else if explicitHost != "" {
		return explicitHost, nil
	} else {
		h, err := transport.GetLocalIP()
		if err != nil {
			return "", err
		}
		return h, nil
	}
}
