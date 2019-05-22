package qBT

import (
	"encoding/json"
	log "github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
)

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func checkAndLog(e error, payload []byte) {
	if e != nil {
		tmpfile, _ := ioutil.TempFile("", "reflection")
		tmpfile.Write(payload)
		log.WithField("filename", tmpfile.Name()).Error("Saved payload in file")
		tmpfile.Close()

		panic(e)
	}
}

type Auth struct {
	LoggedIn bool
	Cookie   http.Cookie
}

type Connection struct {
	Addr *url.URL
	// Hash to ID map. Array index is an ID
	HashIds   []string
	hashIdMap map[string]int
	Tr        *http.Transport
	Client    *http.Client
	Auth      Auth
	MainData  MainData
}

func (q *Connection) Init(baseUrl string) {
	q.HashIds = make([]string, 0)
	q.hashIdMap = make(map[string]int)

	apiAddr, _ := url.Parse("api/v2/")
	parsedBaseAddr, _ := url.Parse(baseUrl)
	q.Addr = parsedBaseAddr.ResolveReference(apiAddr)
}

func (q *Connection) MakeRequestURLWithParam(path string, params map[string]string) string {
	if strings.HasPrefix(path, "/") {
		panic("Invalid API path: " + path)
	}
	parsedPath, err := url.Parse(path)
	check(err)
	u := q.Addr.ResolveReference(parsedPath)
	if len(params) > 0 {
		query := u.Query()
		for key, value := range params {
			query.Set(key, value)
		}
		u.RawQuery = query.Encode()
	}

	return u.String()
}

func (q *Connection) MakeRequestURL(path string) string {
	return q.MakeRequestURLWithParam(path, map[string]string{})
}

func (q *Connection) getTorrentListDirect(category *string) (resp []TorrentsList) {
	params := map[string]string{}
	if category != nil {
		params["category"] = *category
	}
	url := q.MakeRequestURLWithParam("torrents/info", params)
	torrents := q.DoGET(url)

	err := json.Unmarshal(torrents, &resp)
	checkAndLog(err, torrents)
	return
}

func (q *Connection) getTorrentListCached() (resp []TorrentsList) {
	url := q.MakeRequestURL("sync/maindata")
	mainData := q.DoGET(url)

	err := json.Unmarshal(mainData, &q.MainData)
	checkAndLog(err, mainData)
	for hash, torrentData := range *q.MainData.Torrents {
		torrentData.Hash = hash
		resp = append(resp, torrentData)
	}
	return
}

func (q *Connection) GetTorrentList(category *string) (resp []TorrentsList) {
	resp = q.getTorrentListDirect(category)
	q.UpdateIDs(resp)
	return
}

func (q *Connection) AddNewCategory(category string) {
	url := q.MakeRequestURLWithParam("torrents/createCategory", map[string]string{"category": category})
	q.DoGET(url)
}

func (q *Connection) GetPropsGeneral(hash string) (propGeneral PropertiesGeneral) {
	propGeneralURL := q.MakeRequestURLWithParam("torrents/properties", map[string]string{"hash": hash})
	propGeneralRaw := q.DoGET(propGeneralURL)

	err := json.Unmarshal(propGeneralRaw, &propGeneral)
	checkAndLog(err, propGeneralRaw)
	return
}

func (q *Connection) GetPropsTrackers(hash string) (trackers []PropertiesTrackers) {
	trackersURL := q.MakeRequestURLWithParam("torrents/trackers", map[string]string{"hash": hash})
	trackersRaw := q.DoGET(trackersURL)

	err := json.Unmarshal(trackersRaw, &trackers)

	checkAndLog(err, trackersRaw)
	return
}

func (q *Connection) GetPiecesStates(hash string) (pieces []byte) {
	piecesURL := q.MakeRequestURLWithParam("torrents/pieceStates", map[string]string{"hash": hash})
	piecesRaw := q.DoGET(piecesURL)

	err := json.Unmarshal(piecesRaw, &pieces)

	checkAndLog(err, piecesRaw)
	return
}

func (q *Connection) GetPreferences() (pref Preferences) {
	prefURL := q.MakeRequestURL("app/preferences")
	prefRaw := q.DoGET(prefURL)

	err := json.Unmarshal(prefRaw, &pref)
	checkAndLog(err, prefRaw)
	return
}

func (q *Connection) GetTransferInfo() (info TransferInfo) {
	infoURL := q.MakeRequestURL("transfer/info")
	infoRaw := q.DoGET(infoURL)

	err := json.Unmarshal(infoRaw, &info)
	checkAndLog(err, infoRaw)
	return
}

func (q *Connection) GetMainData() (info TransferInfo) {
	mainDataURL := q.MakeRequestURL("sync/maindata")
	mainDataRaw := q.DoGET(mainDataURL)

	err := json.Unmarshal(mainDataRaw, &info)
	checkAndLog(err, mainDataRaw)
	return
}

func (q *Connection) GetVersion() string {
	versionURL := q.MakeRequestURL("app/version")
	return string(q.DoGET(versionURL))
}

func (q *Connection) GetPropsFiles(hash string) (files []PropertiesFiles) {
	filesURL := q.MakeRequestURLWithParam("torrents/files", map[string]string{"hash": hash})
	filesRaw := q.DoGET(filesURL)

	err := json.Unmarshal(filesRaw, &files)
	checkAndLog(err, filesRaw)
	return
}

func (q *Connection) DoGET(url string) []byte {
	req, err := http.NewRequest("GET", url, nil)
	check(err)
	req.AddCookie(&q.Auth.Cookie)

	resp, err := q.Client.Do(req)
	check(err)
	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
	return data
}

func (q *Connection) DoPOST(url string, contentType string, body io.Reader) []byte {
	req, err := http.NewRequest("POST", url, body)
	check(err)
	req.Header.Set("Content-Type", contentType)
	req.AddCookie(&q.Auth.Cookie)

	resp, err := q.Client.Do(req)
	check(err)
	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
	return data
}

func (q *Connection) PostForm(url string, data url.Values) []byte {
	return q.DoPOST(url, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
}

func (q *Connection) GetHashForId(id int) string {
	if len(q.HashIds) >= id {
		return q.HashIds[id-1]
	} else {
		return "a"
	}
}

func (q *Connection) GetHashNum() int {
	return len(q.HashIds)
}

func (q *Connection) GetIdOfHash(hash string) (int, bool) {
	value, ok := q.hashIdMap[hash]
	return value + 1, ok
}

func (q *Connection) Login(username, password string) bool {
	resp, err := http.PostForm(q.MakeRequestURL("auth/login"),
		url.Values{"username": {username}, "password": {password}})
	check(err)
	for _, value := range resp.Cookies() {
		if value != nil {
			cookie := *value
			if cookie.Name == "SID" {
				q.Auth.LoggedIn = true
				q.Auth.Cookie = cookie
				break
			}
		}
	}
	return q.Auth.LoggedIn
}

func (q *Connection) UpdateIDs(torrentsList []TorrentsList) {
	keepHash := make(map[int]interface{})
	addedCount := 0

	for _, torrent := range torrentsList {
		newHash := torrent.Hash
		if index, exists := q.hashIdMap[newHash]; exists {
			keepHash[index] = true
		} else {
			lastIndex := len(q.HashIds)
			q.hashIdMap[newHash] = lastIndex
			keepHash[lastIndex] = true
			q.HashIds = append(q.HashIds, newHash)
			addedCount++
		}
	}
	if addedCount > 0 {
		log.WithField("num", addedCount).Info("Added new hashes to IDs table")
	}

	for i := 0; i < len(q.HashIds); i++ {
		if q.HashIds[i] == "" {
			continue
		}
		if _, exists := keepHash[i]; !exists {
			log.WithField("hash", q.HashIds[i]).Info("Hash disappeared from the torrent list")
			delete(q.hashIdMap, q.HashIds[i])
			q.HashIds[i] = ""
		}
	}
}
