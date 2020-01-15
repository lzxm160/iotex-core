// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package identityset

import (
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/iotex-address/address"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/pkg/log"
)

var sm2keyPortfolio = []string{
	//0 "address": "io1tv9lyhxqs64aft4x975uap6zwl58snuk2jsdxl",
	//"privateKey": "8d8b41f49826b4c57c5f20c1b2e5d69d8f6f80c1d80c03f92ec704e5ffe49df2",
	//"publicKey": "04e4d7558cd44a96a73dfff6ec48233dc040cad1918449cd89e6fd8f99987443dcc2421675583ffc95ab9858fb7a9e08d29d1c5bda266f0c91dacc389fcf0799c8"
	"8d8b41f49826b4c57c5f20c1b2e5d69d8f6f80c1d80c03f92ec704e5ffe49df2",
	//1 "address": "io1cfkn2na9jcyqxq20r8zthn6mxwt9pt7vnu9q9k",
	//"privateKey": "e3b66dcd1e2729a2887a838cab785a628c9534892fd95c21e37610a554278951",
	//"publicKey": "04fcac62bec46f6c3eca4aa1c8777ad60fac86865827c03024f928efd3d312836bf6ee28a80d70fd66389674e56440321273ae35315cb02e768f9db4cf1d95a248"
	"e3b66dcd1e2729a2887a838cab785a628c9534892fd95c21e37610a554278951",
	//2 "address": "io1vn7mv5j9haexw924wxjjnc93hh73a4hnm4hekt",
	//"privateKey": "ddec6250afee7395c16631a514b97405e72d26b19975f893fa84deee4410fe1c",
	//"publicKey": "04abbc078f5f8cb16864f9e68107fe06213de34a7457163d31caf9a5b74e7c3d21a2fb0fd81b40344fb3ea97927bfa5a49c2cfd55bce147afd9a03e62eab260a4f"
	"ddec6250afee7395c16631a514b97405e72d26b19975f893fa84deee4410fe1c",
	//3 "address": "io1mymc7snypye8vp7k8cvlacqphcecuhcl504qns",
	//"privateKey": "e7891557efad949665007cbdbc90a6b4a364efed796751cf2594297cc41a0005",
	//"publicKey": "046fccf152b28a1a315eaa150ae2ae243d52a3bafb8a8748d27209237790ba3a2bad6fe10238f69bce6080391d1450e73ed47c6ec6d56b3e814b5720b20d1805bb"
	"e7891557efad949665007cbdbc90a6b4a364efed796751cf2594297cc41a0005",
	//4 "address": "io1esu5gczgzcg240ljr9ljxkcgyahrmuu2k4rtap",
	//"privateKey": "317d8897fc68bb2a7d6e2dc1c3ef77bff4284a37f7a9bbf80772866b9b6ad880",
	//"publicKey": "04ef02593613c8d2ce7c43782cf2ccc3590d49c618af47f8e40915aa5d4bfd976d8f47282f00c8d46bc1ef5bf18eccdaac5ac434fa0467af295120657fb90521e5"
	"317d8897fc68bb2a7d6e2dc1c3ef77bff4284a37f7a9bbf80772866b9b6ad880",
	//5 "address": "io1r8vurpcuhez840skkwcczmltpqtkg4tcptnrqh",
	//"privateKey": "f96423118790c01ad4021de05de85dbcb22c76bf3156dbba747c61b6ecf46d90",
	//"publicKey": "0466350f102cc9b95b776863107b78450e623cd5625f48de27a6d65272ce1e7a44506201f66e4a15d246bf8e072080c1c10221b766fc62ea592792b319157b98b8"
	"f96423118790c01ad4021de05de85dbcb22c76bf3156dbba747c61b6ecf46d90",
	//6 "address": "io15c7j7wmzhqr46akssp3pwnldaxhj7mk54pv82z",
	//"privateKey": "213d00057b9e1ee0003479baf5d08948272b0915683b9b1c2b45c9379cef2840",
	//"publicKey": "0435bf878c4b6150222660bc09da9a08d7932d8b46218a504a654b3cea63276bba8f6bde4044685b16e3257d6b32886cecd024075e0e06cff20beadf2767c13ad2"
	"213d00057b9e1ee0003479baf5d08948272b0915683b9b1c2b45c9379cef2840",
	//7 "address": "io1cdhq7jesm7ace34hzksv44ffaw4xt8tfyemxm9",
	//"privateKey": "3ebb62a6ffe88b3b35d0b888477f5557916f472ea36347c3d7b7f3c07dc7314a",
	//"publicKey": "044a799928bc10e53fb135d2fda41a1f98f3a74f97360eec7f617fc4c150d97897606cb8046e9209bbffa4bd6c99e5b677f4afb92f413602f4b4c8147249db3ac3"
	"3ebb62a6ffe88b3b35d0b888477f5557916f472ea36347c3d7b7f3c07dc7314a",
	//8 "address": "io145d04f5dvq648fcda8rcmxvpfc8g9vctam2d3n",
	//"privateKey": "048e842f3ea58b7069d28d522ad355a382339cbdc674dbddcc10dcfceb226b34",
	//"publicKey": "040e352d03dc04b27a1e6d34edc574569b4bafe11b50ebf0bffed56be31b3ca516f8d358848cdced55a87fc8337b0ac6a027ab9c953ae51c884491dfbd84ec4acf"
	"048e842f3ea58b7069d28d522ad355a382339cbdc674dbddcc10dcfceb226b34",
	//9 "address": "io1ejcz6djrhpjqxmhdd80anu473jdtlcj5kzlqeu",
	//"privateKey": "2a1d0d0e4ec61d12bf54f45f1e757df7a838f8157fa87ed0edc2711315bd8bc2",
	//"publicKey": "04294267c15ee2e5a433e186e03ee5bcd99db20154e4c51538ef5c5419f96da0e1598b0d610540b04256465f1c11fb78bae8935e26525db8ac9986a77c8e100527"
	"2a1d0d0e4ec61d12bf54f45f1e757df7a838f8157fa87ed0edc2711315bd8bc2",
	//10 "address": "io1y924p2glqvglx5gad29usl3tfhr4rkzzujwqrj",
	//"privateKey": "63f381c783b70c5a57e7252a6438c8fcf43871064c0a4deb3c71f30e65d9967d",
	//"publicKey": "043656ad2892744676c6335fc4a59048f8ca2322580983566228e4ea6105b8ac633ad2171823a6d02c83c2f62142c9589f2f6fbd74204b729b687baf86fe805500"
	"63f381c783b70c5a57e7252a6438c8fcf43871064c0a4deb3c71f30e65d9967d",
	//11 "address": "io14n7asm8wz32dgh5g60jauqjmuttq9769hyytc0",
	//"privateKey": "12cadc4ff118a03c389b721d48964d188292124feddff365232f6a838dacfe9b",
	//"publicKey": "040f7bf59960be8598f80287b8942e624821f88ff41d6df11d319aaf71c46125641c4a149de872800525ce5feb8c82216c9f5564d84f01c541de2c7ec974d20365"
	"12cadc4ff118a03c389b721d48964d188292124feddff365232f6a838dacfe9b",
	//12 "address": "io13a0rlwxq02vjjpd3qvj2lzm759staeshshd356",
	//"privateKey": "0a53ec703136acab14ed865b76d88623220f56ed192cbbd7a8343c5392fb462d",
	//"publicKey": "0463d9da4758013830473d972930631d97637eaaac8cab7eb51d8a5762882a2f1607cec03d1ab8cdd41f3570ad4ff9834b4f77663c07ee1a9da584c21b3fb014dc"
	"0a53ec703136acab14ed865b76d88623220f56ed192cbbd7a8343c5392fb462d",
	//13 "address": "io1hxp345ag68dtdr4k6hr47aksa3jqgkxg8lqjwd",
	//"privateKey": "045c0d829dd76b4192dcfb14e7c573951c307944021be470f9bb28f76501e29c",
	//"publicKey": "04f2915e37d4c180d78b528bf6b5920559eaa5d5b7a4e3da96949a6b9bcdf224a5fe1dbf0b9de9ae7c307ed9cf45a2331e545c102ef846103153a9f3a37ae6e47a"
	"045c0d829dd76b4192dcfb14e7c573951c307944021be470f9bb28f76501e29c",
	//14 "address": "io1kayp58k6pldg6yrx95mgp92n5kdua8gadnr7dh",
	//"privateKey": "ced344c0626a00f5faaf5226bb42f5ab58cbc997f77002132ae3ae08a9dd9336",
	//"publicKey": "0423da83eb6c41ef2c22d59998e4530a062b5d5d722c4400c85f7981f99b6e26b72973f784b4d7f151addf213b395194b6d0b591def2db2cde786df46bfc16f145"
	"ced344c0626a00f5faaf5226bb42f5ab58cbc997f77002132ae3ae08a9dd9336",
	//15 "address": "io1a9yz9um63cvnt55frz4zgh5sqmkveztdk54j5f",
	//"privateKey": "3eb0381f605e68264460362044394aa98394789746f89ccb83b144bde3b12fa5",
	//"publicKey": "04fed2100817a262a1b3b4c2a5770a683af6ede79cd82da59568c4c6cd16027e1810655fc696a913cfb63aed3e2c269dda885b22299cdea66ccfa1025cf283e69c"
	"3eb0381f605e68264460362044394aa98394789746f89ccb83b144bde3b12fa5",
	//16 "address": "io16l7zc7u0e2ugp65wylz3y2hvwn9gpx23n8d4se",
	//"privateKey": "4963be7809fb93d19039e9930a59943e737dd7312e3b10ecca06896e3f196888",
	//"publicKey": "04c1977fb11e305e33c1d9c0cc581f55bcbd77362a5dcac5e3de8797fd3e07d2cc35964cf090f1e3285784195ae7724a631420f8c24aec60aaa5f2e6783623f29d"
	"4963be7809fb93d19039e9930a59943e737dd7312e3b10ecca06896e3f196888",
	//17 "address": "io1r448ua0m6afgmnpr2sqs0l8hknwuefq09k60gc",
	//"privateKey": "ca09a6bf30aad957b4f61b26be1f429c7419bbbbdd0f5f5f145435a8312275b2",
	//"publicKey": "04deca635b8606f65728936b140c1a9eff8c2273b9312fb24f6dce49274dffc16909ff784fc85a59330792b2c05e46e746d16dfd62f58ad2af201abb31e708fd38"
	"ca09a6bf30aad957b4f61b26be1f429c7419bbbbdd0f5f5f145435a8312275b2",
	//18 "address": "io1z9ywcr720nkpuvyq3ak4njnjdcrpyym3lrcyfc",
	//"privateKey": "fa93794e542229ce2d4f826b83e2597bb4684b73a9460e09640f1c67513bb816",
	//"publicKey": "04db6a66da16623435c24c77050ebb17fe7641d5f20bbbaf701832d475e8ff5f48ed7652f1867f5af57da1a4074fc43f7fb8d724f3571b362aab3d786c0ba5b6d9"
	"fa93794e542229ce2d4f826b83e2597bb4684b73a9460e09640f1c67513bb816",
	//19 "address": "io1p494rk4rc2elr2egzrujdr25ql7yenmm547t5a",
	//"privateKey": "17b840565383a66c95e119a67e2833c824f494704b86c397cb6d784f57d69bc5",
	//"publicKey": "04893a46e741e334caea0a084ebee901ffd61c81aa87d54286d5c262d9180e9cde967082a500df1a29b98c61a63fe7a6bd57ab821a46006053a4b45dc8de3cb65c"
	"17b840565383a66c95e119a67e2833c824f494704b86c397cb6d784f57d69bc5",
	//20 "address": "io1dq578p8pp5chtk5lx4r99pzdda600twhnwjcg8",
	//"privateKey": "6f6efd547758dd7177cf8ad371fe7bccde0b44d90afbf98dcc28b1467ec87585",
	//"publicKey": "04b22efcad048353e7082f6de18c657c5d25e27176e1dc841e1274e3b9e005d747bdb7efd2d900bfa35fe2d774b45169b49481d4c930a071bc96b350615aae5206"
	"6f6efd547758dd7177cf8ad371fe7bccde0b44d90afbf98dcc28b1467ec87585",
	//21 "address": "io13j2dyw42auhfegud7asxxkvrklqau3kkq247hw",
	//"privateKey": "664d1a5744558f732cb18aa3aa85ec6dc9804b1c67b0484e14b8c8e93f283f38",
	//"publicKey": "0414e9c7baf394bb87e772536fcc514ad0f799682862c00b39a476e77072a968d815c0bff62b36a99289e8d23cc658bf4104f76f25909c264a61b5ddfe14d9d0ee"
	"664d1a5744558f732cb18aa3aa85ec6dc9804b1c67b0484e14b8c8e93f283f38",
	//22 "address": "io1vmajsk04s93lyfdevnl6wxtp85nt7p86lsltjp",
	//"privateKey": "eca2111e43a43010a03d6f5a303edff50d87ba6ff057d700cf50eab462af9a7b",
	//"publicKey": "046b1a5c7559c79138d44e8f8020a10ff92a9e4058018b88fac56b795a23e5b9ae436c0c81e6d172cfbf3253507a30b660fe957c2fa70ac548b017b304f99344a5"
	"eca2111e43a43010a03d6f5a303edff50d87ba6ff057d700cf50eab462af9a7b",
	//23 "address": "io1dj5h26tqlwplqy8tn3nwwjnt7kqrzgyxkjs064",
	//"privateKey": "0aa7695bd13c8144cda0d82f42af35c4bea1fc275348c8285ffe47cee8697eeb",
	//"publicKey": "046f1afc64b2617b734edad6c649200dbe7f35dccd1426985d8bc00d33f2c25874ec5c0b3a7a3ddf80b26be761e441b375aa888e66439b67252ee7dc69c0084462"
	"0aa7695bd13c8144cda0d82f42af35c4bea1fc275348c8285ffe47cee8697eeb",
	////24 "address": "io1amrchzxjyjfsa4w5n3p4ae3v052zqjsjmvaz2h",
	////"privateKey": "cefd2e41a72931f05f5ecdbbc0f0bb851b9d6dcf7155aa9bf491f01782b453f1",
	////"publicKey": "04be6fa7cd414057c8bc9f57faffbf14be2b4377568b7dbb4387bb4588bb0c2d40a2d4655b63ff4567a4743b5c07082932f8161863dcb92ccc014d9fda04e5d324"
	//"cefd2e41a72931f05f5ecdbbc0f0bb851b9d6dcf7155aa9bf491f01782b453f1",
	////25 "address": "io1t4kfrrxzvrnchuy5f3unt487cetzjjkfr50tgp",
	////"privateKey": "0bf442918170624c97f4036ed0f801c171ec180628cd79fec967b209fddd1583",
	////"publicKey": "04fcd347831aafa54caf2afc7211106b28b177086b89f2a47af30733977fe2cbe79be2f12359171ef2c7031e5636332271c96e72ee4d20bffe21677003db2b5cd9"
	//"0bf442918170624c97f4036ed0f801c171ec180628cd79fec967b209fddd1583",
	////26 "address": "io1xplfpykr3kpjetteapqgg9my74jt7533fd6lzj",
	////"privateKey": "34af514aedda811fd04b2adf0ea1394808a6d733195224e23c54b599a9e254dc",
	////"publicKey": "04330d55d57f640a4e37d5f161ad2edb592ada359fed0240a8ac58b94226be003fabfd0fdc8c654ede7cad1288673673a8d1a191b41e1d823169f7aa2e229790d5"
	//"34af514aedda811fd04b2adf0ea1394808a6d733195224e23c54b599a9e254dc",
}

// Sm2Size returns the size of the address
func Sm2Size() int {
	return 24 //27 is origin size before add last 8 private key,len(keyPortfolio)
}

// Sm2PrivateKey returns the i-th identity's private key
func Sm2PrivateKey(i int) crypto.PrivateKey {
	sk, err := crypto.HexStringToPrivateKey(sm2keyPortfolio[i], true)
	if err != nil {
		log.L().Panic(
			"Error when decoding private key string",
			zap.String("keyStr", sm2keyPortfolio[i]),
			zap.Error(err),
		)
	}
	return sk
}

// Sm2Address returns the i-th identity's address
func Sm2Address(i int) address.Address {
	sk := Sm2PrivateKey(i)
	addr, err := address.FromBytes(sk.PublicKey().Hash())
	if err != nil {
		log.L().Panic("Error when constructing the address", zap.Error(err))
	}
	return addr
}
