package phigros

import "time"

/*
类型对照关系:
内置类型:对应读取函数:存档所需类型
	Bool:Bool:bool
	String:String:string
	Float32:Float32:float32
	Int16:Short:Short
	Int32:Int32:Int
	Uint8:Byte1:Byte
	Uint16:VarShort:VarInt
注:uint8==byte
*/

type SaveData struct {
	GameRecord   GameRecord   `json:"gameRecord"`
	GameKey      GameKey      `json:"gameKey"`
	GameProgress GameProgress `json:"gameProgress"`
	User         User         `json:"user"`
	Settings     Settings     `json:"settings"`
}

type Settings struct {
	ChordSupport      bool    `json:"chordSupport"`
	FcAPIndicator     bool    `json:"fcAPIndicator"`
	EnableHitSound    bool    `json:"enableHitSound"`
	LowResolutionMode bool    `json:"lowResolutionMode"`
	DeviceName        string  `json:"deviceName"`
	Bright            float32 `json:"bright"`
	MusicVolume       float32 `json:"musicVolume"`
	EffectVolume      float32 `json:"effectVolume"`
	HitSoundVolume    float32 `json:"hitSoundVolume"`
	SoundOffset       float32 `json:"soundOffset"`
	NoteScale         float32 `json:"noteScale"`
}

type User struct {
	ShowPlayerId bool   `json:"showPlayerId"`
	SelfIntro    string `json:"selfIntro"`
	Avatar       string `json:"avatar"`
	Background   string `json:"background"`
}

type GameKeyV1 struct {
	LanotaReadKeys uint8 `json:"lanotaReadKeys"`
}

type GameKeyV2 struct {
	CamelliaReadKey bool `json:"camelliaReadKey"`
}

type GameKeyEntry [5]byte

type GameKey struct {
	Version   byte                    `json:"version" phi:"-"`
	Map       map[string]GameKeyEntry `json:"map" phi:"map"`
	GameKeyV1 `phi:"since=1"`
	GameKeyV2 `phi:"since=2"`
	Overflow  []byte `json:"overflow,omitempty" phi:"overflow"`
}

type GameProgressV1 struct {
	IsFirstRun                 bool      `json:"isFirstRun"`
	LegacyChapterFinished      bool      `json:"legacyChapterFinished"`
	AlreadyShowCollectionTip   bool      `json:"alreadyShowCollectionTip"`
	AlreadyShowAutoUnlockINTip bool      `json:"alreadyShowAutoUnlockINTip"`
	Completed                  string    `json:"completed"`
	SongUpdateInfo             uint8     `json:"songUpdateInfo"`
	ChallengeModeRank          int16     `json:"challengeModeRank"`
	Money                      [5]uint16 `json:"money"`
	UnlockFlagOfSpasmodic      uint8     `json:"unlockFlagOfSpasmodic"`
	UnlockFlagOfIgallta        uint8     `json:"unlockFlagOfIgallta"`
	UnlockFlagOfRrharil        uint8     `json:"unlockFlagOfRrharil"`
	FlagOfSongRecordKey        uint8     `json:"flagOfSongRecordKey"`
}

type GameProgressV2 struct {
	RandomVersionUnlocked uint8 `json:"randomVersionUnlocked"`
}

type GameProgressV3 struct {
	Chapter8UnlockBegin       bool  `json:"chapter8UnlockBegin"`
	Chapter8UnlockSecondPhase bool  `json:"chapter8UnlockSecondPhase"`
	Chapter8Passed            bool  `json:"chapter8Passed"`
	Chapter8SongUnlocked      uint8 `json:"chapter8SongUnlocked"`
}

type GameProgress struct {
	Version        byte `json:"version" phi:"-"`
	GameProgressV1 `phi:"since=1"`
	GameProgressV2 `phi:"since=2"`
	GameProgressV3 `phi:"since=3"`
	Overflow       []byte `json:"overflow,omitempty" phi:"overflow"`
}

type SongLevel struct {
	Score int32   `json:"score"`
	Acc   float32 `json:"acc"`
	FC    bool    `json:"fc" phi:"-"`
}

type SongRecord struct {
	EZ *SongLevel `json:"EZ,omitempty"`
	HD *SongLevel `json:"HD,omitempty"`
	IN *SongLevel `json:"IN,omitempty"`
	AT *SongLevel `json:"AT,omitempty"`
}

type GameRecord map[string]SongRecord

type ScoreAcc struct {
	Score      int     `json:"score"`
	Acc        float32 `json:"acc"`
	Level      string  `json:"level"`
	Fc         bool    `json:"fc"`
	SongId     string  `json:"songId"`
	Difficulty float32 `json:"difficulty"`
	Rks        float32 `json:"rks" phi:"-"`
}

// net struct
type UserMe struct {
	ACL struct {
		NAMING_FAILED struct {
			Write bool `json:"write"`
			Read  bool `json:"read"`
		} `json:"*"`
	} `json:"ACL"`
	AuthData struct {
		Taptap struct {
			AccessToken  string `json:"access_token"`
			Avatar       string `json:"avatar"`
			Kid          string `json:"kid"`
			MacAlgorithm string `json:"mac_algorithm"`
			MacKey       string `json:"mac_key"`
			Name         string `json:"name"`
			Openid       string `json:"openid"`
			TokenType    string `json:"token_type"`
			Unionid      string `json:"unionid"`
		} `json:"taptap"`
	} `json:"authData"`
	Avatar              string    `json:"avatar"`
	CreatedAt           time.Time `json:"createdAt"`
	EmailVerified       bool      `json:"emailVerified"`
	MobilePhoneVerified bool      `json:"mobilePhoneVerified"`
	Nickname            string    `json:"nickname"`
	ObjectID            string    `json:"objectId"`
	SessionToken        string    `json:"sessionToken"`
	ShortID             string    `json:"shortId"`
	UpdatedAt           time.Time `json:"updatedAt"`
	Username            string    `json:"username"`
}

type GameSave struct {
	Results []struct {
		CreatedAt time.Time `json:"createdAt"`
		GameFile  struct {
			Type      string    `json:"__type"`
			Bucket    string    `json:"bucket"`
			CreatedAt time.Time `json:"createdAt"`
			Key       string    `json:"key"`
			MetaData  struct {
				Checksum string `json:"_checksum"`
				Prefix   string `json:"prefix"`
				Size     int    `json:"size"`
			} `json:"metaData"`
			MimeType  string    `json:"mime_type"`
			Name      string    `json:"name"`
			ObjectID  string    `json:"objectId"`
			Provider  string    `json:"provider"`
			UpdatedAt time.Time `json:"updatedAt"`
			URL       string    `json:"url"`
		} `json:"gameFile"`
		ModifiedAt struct {
			Type string    `json:"__type"`
			Iso  time.Time `json:"iso"`
		} `json:"modifiedAt"`
		Name      string    `json:"name"`
		ObjectID  string    `json:"objectId"`
		Summary   string    `json:"summary"`
		UpdatedAt time.Time `json:"updatedAt"`
		User      struct {
			Type      string `json:"__type"`
			ClassName string `json:"className"`
			ObjectID  string `json:"objectId"`
		} `json:"user"`
	} `json:"results"`
}

type PlayerInfo struct {
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Avatar    string    `json:"avatar"`
}

type UserRecord struct {
	Session    string      `json:"session"`
	PlayerInfo *PlayerInfo `json:"playerInfo"`
	ScoreAcc   []ScoreAcc  `json:"scoreAcc"`
	GameRecord *GameRecord `json:"-"`
	Summary    *Summary    `json:"summary"`
}

type RespCode struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

type Summary struct {
	SaveVersion       byte               `json:"saveVersion"`
	ChallengeModeRank int16              `json:"challengeModeRank"`
	Rks               float32            `json:"rks"`
	GameVersion       uint16             `json:"gameVersion"`
	Avatar            string             `json:"avatar"`
	ScoreAcc          [4]SummaryScoreAcc `json:"scoreAcc"`
	ChalID            int16              `json:"chalID" phi:"-"`
	Chalnum           string             `json:"chalnum" phi:"-"`
}

type SummaryScoreAcc struct {
	Cleared   int16 `json:"cleared"`
	FullCombo int16 `json:"fullCombo"`
	Phi       int16 `json:"phi"`
}
