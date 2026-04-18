package phigros

import (
	"archive/zip"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/tidwall/gjson"
)

type Serializable interface {
	Settings | User | Summary | GameKey | GameProgress | GameRecord
}

type phiFieldOptions struct {
	Skip     bool
	Map      bool
	Overflow bool
	Since    int
}

func parsePhiFieldOptions(tag string) phiFieldOptions {
	if tag == "-" {
		return phiFieldOptions{Skip: true}
	}

	var options phiFieldOptions
	if tag == "" {
		return options
	}

	for _, part := range strings.Split(tag, ",") {
		switch {
		case part == "map":
			options.Map = true
		case part == "overflow":
			options.Overflow = true
		case strings.HasPrefix(part, "since="):
			value, _ := strconv.Atoi(strings.TrimPrefix(part, "since="))
			options.Since = value
		}
	}
	return options
}

func Unmarshal[T Serializable](in []byte) *T {
	value, _ := decodeSerializable[T](in, 0)
	return value
}

func Marshal[T Serializable](in *T) []byte {
	value, _ := encodeSerializable(in)
	return value
}

func decodeSerializable[T Serializable](in []byte, version byte) (*T, error) {
	var value T
	rv := reflect.ValueOf(&value).Elem()
	if version != 0 {
		setVersionField(rv, version)
	}
	reader := NewBytesReader(in)
	if err := decodeValue(rv, reader); err != nil {
		return nil, err
	}
	return &value, nil
}

func encodeSerializable[T Serializable](in *T) ([]byte, error) {
	buff := new(Buff)
	if err := encodeValue(reflect.ValueOf(in).Elem(), buff); err != nil {
		return nil, err
	}
	return buff.Bytes.Bytes(), nil
}

func setVersionField(rv reflect.Value, version byte) {
	field := rv.FieldByName("Version")
	if field.IsValid() && field.CanSet() && field.Kind() == reflect.Uint8 {
		field.SetUint(uint64(version))
	}
}

func currentVersion(rv reflect.Value) int {
	field := rv.FieldByName("Version")
	if !field.IsValid() || field.Kind() != reflect.Uint8 {
		return 0
	}
	return int(field.Uint())
}

func safeDecode(fn func() error) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("decode panic: %v", recovered)
		}
	}()
	return fn()
}

func decodeValue(rv reflect.Value, reader *Bytes) error {
	switch rv.Kind() {
	case reflect.Bool:
		rv.SetBool(reader.ReadBool())
	case reflect.String:
		rv.SetString(reader.ReadString())
	case reflect.Float32:
		rv.SetFloat(float64(reader.ReadFloat32()))
	case reflect.Int16:
		rv.SetInt(int64(reader.ReadShort()))
	case reflect.Int32:
		rv.SetInt(int64(reader.ReadInt32()))
	case reflect.Uint8:
		rv.SetUint(uint64(reader.ReadByte1()))
	case reflect.Uint16:
		rv.SetUint(uint64(reader.ReadVarShort()))
	case reflect.Array:
		for i := 0; i < rv.Len(); i++ {
			if err := decodeValue(rv.Index(i), reader); err != nil {
				return err
			}
		}
	case reflect.Struct:
		return decodeStruct(rv, reader)
	case reflect.Map:
		return decodeMap(rv, reader)
	default:
		return fmt.Errorf("unsupported kind %s", rv.Kind())
	}
	return nil
}

func decodeStruct(rv reflect.Value, reader *Bytes) error {
	rt := rv.Type()
	version := currentVersion(rv)

	for i := 0; i < rv.NumField(); i++ {
		field := rv.Field(i)
		structField := rt.Field(i)
		options := parsePhiFieldOptions(structField.Tag.Get("phi"))
		if options.Skip {
			continue
		}
		if options.Since > 0 && version < options.Since {
			continue
		}
		if options.Overflow {
			field.SetBytes(append([]byte(nil), reader.Data[reader.ptr:]...))
			reader.ptr = len(reader.Data)
			reader.bit = 0
			continue
		}

		snapshotPtr, snapshotBit := reader.ptr, reader.bit
		readField := func() error {
			if options.Map {
				return decodeMap(field, reader)
			}
			return decodeValue(field, reader)
		}

		if options.Since > 0 {
			if err := safeDecode(readField); err != nil {
				reader.ptr, reader.bit = snapshotPtr, snapshotBit
				continue
			}
			continue
		}

		if err := safeDecode(readField); err != nil {
			return fmt.Errorf("%s: %w", structField.Name, err)
		}
	}

	reader.Alignment()
	return nil
}

func decodeMap(rv reflect.Value, reader *Bytes) error {
	if rv.Type().Key().Kind() != reflect.String {
		return fmt.Errorf("unsupported map key type %s", rv.Type().Key())
	}
	if rv.IsNil() {
		rv.Set(reflect.MakeMap(rv.Type()))
	}

	count := int(reader.ReadVarShort())
	for i := 0; i < count; i++ {
		key := reader.ReadString()
		blockLength := int(reader.ReadByte1())
		next := reader.ptr + blockLength

		elem, err := decodeMapEntry(rv.Type().Elem(), reader)
		if err != nil {
			return fmt.Errorf("map[%s]: %w", key, err)
		}
		if isGameRecordEntryType(rv.Type().Elem()) {
			key = strings.TrimSuffix(key, ".0")
		}
		rv.SetMapIndex(reflect.ValueOf(key).Convert(rv.Type().Key()), elem)
		reader.ptr = next
		reader.bit = 0
	}
	return nil
}

func decodeMapEntry(elemType reflect.Type, reader *Bytes) (reflect.Value, error) {
	switch {
	case isGameRecordEntryType(elemType):
		return decodeGameRecordEntry(elemType, reader)
	case isByteFlagEntryType(elemType):
		return decodeByteFlagEntry(elemType, reader)
	default:
		return reflect.Value{}, fmt.Errorf("unsupported map element type %s", elemType)
	}
}

func isGameRecordEntryType(elemType reflect.Type) bool {
	if elemType.Kind() != reflect.Struct || elemType.NumField() != len(levels) {
		return false
	}
	for i := 0; i < elemType.NumField(); i++ {
		field := elemType.Field(i)
		if field.Type.Kind() != reflect.Ptr || field.Type.Elem() != reflect.TypeFor[SongLevel]() {
			return false
		}
	}
	return true
}

func decodeGameRecordEntry(elemType reflect.Type, reader *Bytes) (reflect.Value, error) {
	exists := reader.ReadByte1()
	fc := reader.ReadByte1()
	record := reflect.New(elemType).Elem()

	for level := 0; level < elemType.NumField(); level++ {
		if !GetBool(exists, level) {
			continue
		}
		songLevel := reflect.New(elemType.Field(level).Type.Elem())
		if err := decodeValue(songLevel.Elem(), reader); err != nil {
			return reflect.Value{}, err
		}
		fcField := songLevel.Elem().FieldByName("FC")
		if fcField.IsValid() && fcField.CanSet() {
			fcField.SetBool(GetBool(fc, level))
		}
		record.Field(level).Set(songLevel)
	}

	return record, nil
}

func isByteFlagEntryType(elemType reflect.Type) bool {
	return elemType.Kind() == reflect.Array && elemType.Elem().Kind() == reflect.Uint8
}

func decodeByteFlagEntry(elemType reflect.Type, reader *Bytes) (reflect.Value, error) {
	exists := reader.ReadByte1()
	entry := reflect.New(elemType).Elem()
	for i := 0; i < entry.Len(); i++ {
		if GetBool(exists, i) {
			entry.Index(i).SetUint(uint64(reader.ReadByte1()))
		}
	}
	return entry, nil
}

func encodeValue(rv reflect.Value, buff *Buff) error {
	switch rv.Kind() {
	case reflect.Bool:
		buff.SaveBool(rv.Bool())
	case reflect.String:
		buff.SaveString(rv.String())
	case reflect.Float32:
		buff.SaveFloat32(float32(rv.Float()))
	case reflect.Int16:
		buff.SaveShort(int16(rv.Int()))
	case reflect.Int32:
		buff.SaveInt32(int32(rv.Int()))
	case reflect.Uint8:
		buff.SaveByte1(byte(rv.Uint()))
	case reflect.Uint16:
		buff.SaveVarShort(uint16(rv.Uint()))
	case reflect.Array:
		for i := 0; i < rv.Len(); i++ {
			if err := encodeValue(rv.Index(i), buff); err != nil {
				return err
			}
		}
	case reflect.Struct:
		return encodeStruct(rv, buff)
	case reflect.Map:
		return encodeMap(rv, buff)
	default:
		return fmt.Errorf("unsupported kind %s", rv.Kind())
	}
	return nil
}

func encodeStruct(rv reflect.Value, buff *Buff) error {
	rt := rv.Type()
	version := currentVersion(rv)

	for i := 0; i < rv.NumField(); i++ {
		field := rv.Field(i)
		structField := rt.Field(i)
		options := parsePhiFieldOptions(structField.Tag.Get("phi"))
		if options.Skip {
			continue
		}
		if options.Since > 0 && version < options.Since {
			continue
		}
		if options.Overflow {
			buff.SaveBytes(field.Bytes())
			continue
		}
		if options.Map {
			if err := encodeMap(field, buff); err != nil {
				return fmt.Errorf("%s: %w", structField.Name, err)
			}
			continue
		}
		if err := encodeValue(field, buff); err != nil {
			return fmt.Errorf("%s: %w", structField.Name, err)
		}
	}

	buff.Alignment()
	return nil
}

func encodeMap(rv reflect.Value, buff *Buff) error {
	if rv.Type().Key().Kind() != reflect.String {
		return fmt.Errorf("unsupported map key type %s", rv.Type().Key())
	}

	keys := rv.MapKeys()
	sort.Slice(keys, func(i, j int) bool {
		return keys[i].String() < keys[j].String()
	})

	buff.SaveVarShort(uint16(len(keys)))
	for _, key := range keys {
		block := new(Buff)
		if err := encodeMapEntry(rv.MapIndex(key), block); err != nil {
			return fmt.Errorf("map[%s]: %w", key.String(), err)
		}
		payload := block.Bytes.Bytes()
		if len(payload) > 255 {
			return fmt.Errorf("map entry too large: %s", key.String())
		}
		rawKey := key.String()
		if isGameRecordEntryType(rv.Type().Elem()) {
			rawKey += ".0"
		}
		buff.SaveString(rawKey)
		buff.SaveByte1(byte(len(payload)))
		buff.SaveBytes(payload)
	}
	return nil
}

func encodeMapEntry(value reflect.Value, buff *Buff) error {
	switch {
	case isGameRecordEntryType(value.Type()):
		return encodeGameRecordEntry(value, buff)
	case isByteFlagEntryType(value.Type()):
		return encodeByteFlagEntry(value, buff)
	default:
		return fmt.Errorf("unsupported map element type %s", value.Type())
	}
}

func encodeGameRecordEntry(record reflect.Value, buff *Buff) error {
	var exists byte
	var fc byte
	for level := 0; level < record.NumField(); level++ {
		entry := record.Field(level)
		if entry.IsNil() {
			continue
		}
		exists |= 1 << level
		if entry.Elem().FieldByName("FC").Bool() {
			fc |= 1 << level
		}
	}

	buff.SaveByte1(exists)
	buff.SaveByte1(fc)
	for level := 0; level < record.NumField(); level++ {
		entry := record.Field(level)
		if entry.IsNil() {
			continue
		}
		if err := encodeValue(entry.Elem(), buff); err != nil {
			return err
		}
	}
	return nil
}

func encodeByteFlagEntry(entry reflect.Value, buff *Buff) error {
	var exists byte
	for i := 0; i < entry.Len(); i++ {
		if entry.Index(i).Uint() != 0 {
			exists |= 1 << i
		}
	}
	buff.SaveByte1(exists)
	for i := 0; i < entry.Len(); i++ {
		value := entry.Index(i).Uint()
		if value != 0 {
			buff.SaveByte1(byte(value))
		}
	}
	return nil
}

func (song *SongRecord) Set(level int, value *SongLevel) {
	switch level {
	case 0:
		song.EZ = value
	case 1:
		song.HD = value
	case 2:
		song.IN = value
	case 3:
		song.AT = value
	}
}

func (song SongRecord) Get(level int) *SongLevel {
	switch level {
	case 0:
		return song.EZ
	case 1:
		return song.HD
	case 2:
		return song.IN
	case 3:
		return song.AT
	default:
		return nil
	}
}

func (song SongRecord) Each(fn func(level int, levelName string, record *SongLevel)) {
	for level, levelName := range levels {
		record := song.Get(level)
		if record != nil {
			fn(level, levelName, record)
		}
	}
}

func (gameKey GameKey) schemaVersion() byte {
	if gameKey.Version != 0 {
		return gameKey.Version
	}
	if gameKey.CamelliaReadKey || len(gameKey.Overflow) > 0 {
		return 2
	}
	return 1
}

func (gameProgress GameProgress) schemaVersion() byte {
	if gameProgress.Version != 0 {
		return gameProgress.Version
	}
	if len(gameProgress.Overflow) > 0 ||
		gameProgress.Chapter8UnlockBegin ||
		gameProgress.Chapter8UnlockSecondPhase ||
		gameProgress.Chapter8Passed ||
		gameProgress.Chapter8SongUnlocked != 0 {
		return 3
	}
	if gameProgress.RandomVersionUnlocked != 0 {
		return 2
	}
	return 1
}

func ReadZip(path string) (map[string][]byte, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	return readZipFiles(&reader.Reader)
}

func ReadZipBytes(data []byte) (map[string][]byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	return readZipFiles(reader)
}

func readZipFiles(reader *zip.Reader) (map[string][]byte, error) {
	files := make(map[string][]byte, len(reader.File))
	for _, file := range reader.File {
		stream, err := file.Open()
		if err != nil {
			return nil, err
		}
		data, readErr := io.ReadAll(stream)
		closeErr := stream.Close()
		if readErr != nil {
			return nil, readErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		files[file.Name] = data
	}
	return files, nil
}

func Decrypt(in []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(in) == 0 {
		return nil, errors.New("cipherText too short")
	}
	if len(in) == 1 {
		return append([]byte(nil), in...), nil
	}
	if (len(in)-1)%aes.BlockSize != 0 {
		return nil, errors.New("cipherText is not a multiple of the block size")
	}

	plain := make([]byte, len(in)-1)
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(plain, in[1:])
	plain, err = unpad(plain)
	if err != nil {
		return nil, err
	}
	return append([]byte{in[0]}, plain...), nil
}

func Encrypt(in []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(in) == 0 {
		return nil, errors.New("plainText too short")
	}

	plain := append([]byte(nil), in[1:]...)
	plain = pad(plain, aes.BlockSize)
	encrypted := make([]byte, len(plain))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(encrypted, plain)
	return append([]byte{in[0]}, encrypted...), nil
}

func pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	if padding == 0 {
		padding = blockSize
	}
	return append(data, bytes.Repeat([]byte{byte(padding)}, padding)...)
}

func unpad(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("invalid padding size")
	}
	padding := int(data[len(data)-1])
	if padding == 0 || padding > len(data) {
		return nil, errors.New("invalid padding")
	}
	for _, value := range data[len(data)-padding:] {
		if int(value) != padding {
			return nil, errors.New("invalid padding")
		}
	}
	return data[:len(data)-padding], nil
}

func (g GameRecord) Score() []ScoreAcc {
	records := make([]ScoreAcc, 0, len(g)*4)
	for songID, song := range g {
		diff := difficulty[songID]
		song.Each(func(level int, levelName string, record *SongLevel) {
			scoreAcc := ScoreAcc{
				Score:  int(record.Score),
				Acc:    record.Acc,
				Level:  levelName,
				Fc:     record.FC,
				SongId: songID,
			}
			if level < len(diff) {
				scoreAcc.Difficulty = diff[level]
			}
			scoreAcc.Rks = (scoreAcc.Acc - 55) / 45
			scoreAcc.Rks = scoreAcc.Rks * scoreAcc.Rks * scoreAcc.Difficulty
			records = append(records, scoreAcc)
		})
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].Rks > records[j].Rks
	})
	return records
}

// 前19成绩,取最高成绩放第一位
func B19(records []ScoreAcc) []ScoreAcc {
	return BN(records, 19)
}

// 取前n成绩,取最高成绩放第一位
func BN(records []ScoreAcc, n int) []ScoreAcc {
	var maxRecord ScoreAcc
	for _, r := range records {
		if r.Score == 1000000 && r.Difficulty > maxRecord.Difficulty {
			maxRecord = r
		}
	}

	bn := []ScoreAcc{maxRecord}
	if n <= 0 {
		return append(bn, records...)
	}
	if len(records) >= n {
		return append(bn, records[:n]...)
	}
	return append(bn, records...)
}

func ParseSave(path string) (*SaveData, error) {
	files, err := ReadZip(path)
	if err != nil {
		return nil, err
	}
	return parseSaveFiles(files)
}

func ParseSaveBytes(data []byte) (*SaveData, error) {
	files, err := ReadZipBytes(data)
	if err != nil {
		return nil, err
	}
	return parseSaveFiles(files)
}

func parseSaveFiles(files map[string][]byte) (*SaveData, error) {
	required := []string{"gameRecord", "gameKey", "gameProgress", "user", "settings"}
	for _, name := range required {
		if _, ok := files[name]; !ok {
			return nil, fmt.Errorf("zip not include file %s", name)
		}
	}

	decrypted := make(map[string][]byte, len(files))
	for name, data := range files {
		value, err := Decrypt(data)
		if err != nil {
			return nil, fmt.Errorf("decrypt file %s error %s", name, err.Error())
		}
		decrypted[name] = value
	}

	if decrypted["gameRecord"][0] != 0x01 {
		return nil, errors.New("版本号不正确，可能协议已更新。")
	}

	record, err := decodeSerializable[GameRecord](decrypted["gameRecord"][1:], 0)
	if err != nil {
		return nil, err
	}
	gameKey, err := decodeSerializable[GameKey](decrypted["gameKey"][1:], decrypted["gameKey"][0])
	if err != nil {
		return nil, err
	}
	gameProgress, err := decodeSerializable[GameProgress](decrypted["gameProgress"][1:], decrypted["gameProgress"][0])
	if err != nil {
		return nil, err
	}
	settings, err := decodeSerializable[Settings](decrypted["settings"][1:], 0)
	if err != nil {
		return nil, err
	}
	user, err := decodeSerializable[User](decrypted["user"][1:], 0)
	if err != nil {
		return nil, err
	}

	return &SaveData{
		GameRecord:   *record,
		GameKey:      *gameKey,
		GameProgress: *gameProgress,
		User:         *user,
		Settings:     *settings,
	}, nil
}

func MarshalSave(save *SaveData) ([]byte, error) {
	files := map[string][]byte{}

	recordBody, err := encodeSerializable(&save.GameRecord)
	if err != nil {
		return nil, err
	}
	files["gameRecord"], err = Encrypt(append([]byte{0x01}, recordBody...))
	if err != nil {
		return nil, err
	}

	gameKey := save.GameKey
	gameKey.Version = gameKey.schemaVersion()
	keyBody, err := encodeSerializable(&gameKey)
	if err != nil {
		return nil, err
	}
	files["gameKey"], err = Encrypt(append([]byte{gameKey.Version}, keyBody...))
	if err != nil {
		return nil, err
	}

	gameProgress := save.GameProgress
	gameProgress.Version = gameProgress.schemaVersion()
	progressBody, err := encodeSerializable(&gameProgress)
	if err != nil {
		return nil, err
	}
	files["gameProgress"], err = Encrypt(append([]byte{gameProgress.Version}, progressBody...))
	if err != nil {
		return nil, err
	}

	userBody, err := encodeSerializable(&save.User)
	if err != nil {
		return nil, err
	}
	files["user"], err = Encrypt(append([]byte{0x01}, userBody...))
	if err != nil {
		return nil, err
	}

	settingsBody, err := encodeSerializable(&save.Settings)
	if err != nil {
		return nil, err
	}
	files["settings"], err = Encrypt(append([]byte{0x01}, settingsBody...))
	if err != nil {
		return nil, err
	}

	order := []string{"gameKey", "gameProgress", "gameRecord", "settings", "user"}
	var out bytes.Buffer
	writer := zip.NewWriter(&out)
	for _, name := range order {
		header := &zip.FileHeader{
			Name:   name,
			Method: zip.Store,
		}
		entry, err := writer.CreateHeader(header)
		if err != nil {
			_ = writer.Close()
			return nil, err
		}
		if _, err = entry.Write(files[name]); err != nil {
			_ = writer.Close()
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func WriteSave(path string, save *SaveData) error {
	data, err := MarshalSave(save)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// 通过url获取战绩,其余内容丢弃
func ParseStatsByUrl(url string) (*GameRecord, error) {
	data, err := GetGameRecordData(url)
	if err != nil {
		return nil, err
	}
	data, err = Decrypt(data)
	if err != nil {
		return nil, fmt.Errorf("decrypt file gameRecord error %s", err.Error())
	}
	if data[0] != 0x01 {
		return nil, errors.New("版本号不正确，可能协议已更新。")
	}
	return decodeSerializable[GameRecord](data[1:], 0)
}

func ProcessSummary(sum string) *Summary {
	if sum == "" {
		return nil
	}
	data, err := base64.StdEncoding.DecodeString(sum)
	if err != nil {
		return nil
	}

	summary, err := decodeSerializable[Summary](data, 0)
	if err != nil {
		return nil
	}
	summary.ChalID = (summary.ChallengeModeRank - (summary.ChallengeModeRank % 100)) / 100
	summary.Chalnum = strconv.Itoa(int(summary.ChallengeModeRank % 100))
	return summary
}

func GetUserRecordQuickly(session string) (*UserRecord, error) {
	record := UserRecord{Session: session}
	data, err := GetDataFormTap(UserMeUrl, session)
	if err != nil {
		return nil, err
	}
	userMe := gjson.Parse(BytesToString(data))
	record.PlayerInfo = &PlayerInfo{
		Name:      userMe.Get("nickname").String(),
		CreatedAt: userMe.Get("createdAt").Time(),
		UpdatedAt: userMe.Get("updatedAt").Time(),
		Avatar:    userMe.Get("avatar").String(),
	}

	data, err = GetDataFormTap(SaveUrl, session)
	if err != nil {
		return nil, err
	}
	saveMeta := gjson.Parse(BytesToString(data))
	record.GameRecord, _ = ParseStatsByUrl(saveMeta.Get("results.0.gameFile.url").String())
	if record.GameRecord != nil {
		record.ScoreAcc = record.GameRecord.Score()
	}
	record.Summary = ProcessSummary(saveMeta.Get("results.0.summary").String())
	return &record, nil
}
