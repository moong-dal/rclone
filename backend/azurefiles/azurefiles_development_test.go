package azurefiles

import (
	"bytes"
	"context"
	"crypto/md5"
	"io"
	"math/rand"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azfile/file"
	"github.com/rclone/rclone/fs"
	"github.com/stretchr/testify/assert"
)

// TODO: new object dir cases
// TODO: set modtime on directories

func testNewObjectErrorOnObjectNotExisting(t *testing.T, c *Client) {
	_, err := c.NewObject(context.TODO(), "somefilethatdoesnotexist.txt")
	assert.Error(t, err)
}

func testNewObjectNoErrorIfObjectExists(t *testing.T, c *Client) {
	_, err := c.NewObject(context.TODO(), pre_existing_file_name)
	assert.NoError(t, err)
}

func testSetModTimeNoError(t *testing.T, c *Client) {
	obj, err := c.NewObject(context.TODO(), pre_existing_file_name)
	assert.NoError(t, err)
	randomTime := time.Date(1990+rand.Intn(20), time.December, rand.Intn(31), 0, 0, 0, 0, time.UTC)
	setModTimeErr := obj.SetModTime(context.TODO(), randomTime)
	assert.NoError(t, setModTimeErr)
}

func testSetModTimeStepWise(t *testing.T, c *Client) {
	fc := c.RootDirClient.NewFileClient(pre_existing_file_name)
	metaData := make(map[string]*string)
	someString := "1_" + randomString(10)
	metaData["a"] = &someString
	metaDataOptions := file.SetMetadataOptions{
		Metadata: metaData,
	}
	setMetadataResp, err := fc.SetMetadata(context.TODO(), &metaDataOptions)
	assert.NoError(t, err)
	t.Logf("%v", setMetadataResp)
}

// TODO: test put object in an inner directory
func testPutObject(t *testing.T, c *Client) {

	in, src := randomPuttableObject(randomString(10) + ".txt")
	obj, err := c.Put(context.TODO(), in, src)
	assert.NoError(t, err)
	assert.Equal(t, obj.ModTime(context.TODO()), src.ModTime(context.TODO()))
}

func testListDir(t *testing.T, c *Client) {
	des, err := c.List(context.TODO(), "")
	assert.NoError(t, err)

	t.Run("list contains pre existing files", func(t *testing.T) {
		assert.Contains(t, dirEntriesBases(des), pre_existing_file_name)
	})

	t.Run("subdir contents", func(t *testing.T) {
		des, err := c.List(context.TODO(), pre_existing_dir)
		assert.NoError(t, err)
		assert.Contains(t, dirEntriesBases(des), file_in_pre_existing_dir)
	})

	// TODO: listing contents of dir that does not exist

}

func testMkDir(t *testing.T, c *Client) {
	dirName := "mkDirTest_" + randomString(10)
	err := c.Mkdir(context.TODO(), dirName)
	assert.NoError(t, err)

	t.Run("dir shows up in listDir", func(t *testing.T) {
		des, err := c.List(context.TODO(), "")
		assert.NoError(t, err)
		assert.Contains(t, dirEntriesBases(des), dirName)
	})

	t.Run("nested dir where parent does not exist", func(t *testing.T) {
		parent := "mkDirTest_" + randomString(10)
		child := "mkDirTest_" + randomString(10)
		fullPath := strings.Join([]string{parent, child}, pathSeparator)
		err := c.Mkdir(context.TODO(), fullPath)
		assert.NoError(t, err)
		rootDes, rootListErr := c.List(context.TODO(), "")
		assert.NoError(t, rootListErr)
		assert.Contains(t, dirEntriesBases(rootDes), parent, "presence of parent directory in root")

		parentDes, parentListErr := c.List(context.TODO(), parent)
		assert.NoError(t, parentListErr)
		assert.Contains(t, dirEntriesBases(parentDes), child, "presence of child directory in parent")
	})

	t.Run("subdir where parent exists", func(t *testing.T) {
		subdirName := "mkDirTest_" + randomString(10)
		fullPath := strings.Join([]string{dirName, subdirName}, pathSeparator)
		err := c.Mkdir(context.TODO(), fullPath)
		assert.NoError(t, err)

		des, err := c.List(context.TODO(), dirName)
		assert.NoError(t, err)
		assert.Contains(t, dirEntriesBases(des), subdirName, "presence of subDir in dirName")
	})

	// t.Run("creating a directory inside existing subdir", func(t *testing.T) {
	// 	dirName := "mkDirTest_" + randomString(10)
	// 	path := filepath.Join(pre_existing_dir, dirName)
	// 	err := c.Mkdir(context.TODO(), path)
	// 	assert.NoError(t, err)

	// 	des, err := c.List(context.TODO(), pre_existing_dir)
	// 	assert.NoError(t, err)
	// 	assert.Contains(t, dirEntriesBases(des), dirName)
	// })

	t.Run("no error when directory already exists", func(t *testing.T) {
		err := c.Mkdir(context.TODO(), pre_existing_dir)
		assert.NoError(t, err)
	})

	// TODO: what happens if parent path does not exist
}

func testRmDir(t *testing.T, c *Client) {
	dirToBeRemoved := "rmdirTest_" + randomString(10)
	err := c.Mkdir(context.TODO(), dirToBeRemoved)
	assert.NoError(t, err)
	err = c.Rmdir(context.Background(), dirToBeRemoved)
	assert.NoError(t, err)
	des, err := c.List(context.TODO(), "")
	assert.NoError(t, err)
	assert.NotContains(t, dirEntriesBases(des), dirToBeRemoved)

	t.Run("remove subdir", func(t *testing.T) {
		parentDir := pre_existing_dir
		tempDirName := "rmdirTest_" + randomString(10)
		dirToBeRemoved := filepath.Join(parentDir, tempDirName)
		err := c.Mkdir(context.TODO(), dirToBeRemoved)
		assert.NoError(t, err)
		err = c.Rmdir(context.Background(), dirToBeRemoved)
		assert.NoError(t, err)
	})

	// TODO: assert the exact error returned when rmdir fails
	t.Run("rmdir must fail if directory has contents", func(t *testing.T) {
		tempDir := "rmdirTest_" + randomString(10)
		err := c.Mkdir(context.TODO(), tempDir)
		assert.NoError(t, err)
		fileName := randomString(10) + ".txt"
		filePath := filepath.Join(tempDir, fileName)
		in, src := randomPuttableObject(filePath)
		_, err = c.Put(context.TODO(), in, src, nil)
		assert.NoError(t, err)
		err = c.Rmdir(context.TODO(), filePath)
		assert.Error(t, err)
	})

}

func testRemove(t *testing.T, c *Client) {
	fileName := "testRemove_" + randomString(10) + ".txt"
	in, src := randomPuttableObject(fileName)
	obj, err := c.Put(context.TODO(), in, src, nil)
	assert.NoError(t, err)
	err = obj.Remove(context.TODO())
	assert.NoError(t, err)
	des, err := c.List(context.TODO(), "")
	assert.NoError(t, err)
	assert.NotContains(t, dirEntriesBases(des), fileName)

	t.Run("works on files inside subdirectory", func(t *testing.T) {
		fileName := "testRemove_" + randomString(10) + ".txt"
		filePath := filepath.Join(pre_existing_dir, fileName)
		in, src := randomPuttableObject(filePath)
		obj, err := c.Put(context.TODO(), in, src, nil)
		assert.NoError(t, err)
		err = obj.Remove(context.TODO())
		assert.NoError(t, err)
		des, err := c.List(context.TODO(), pre_existing_dir)
		assert.NoError(t, err)
		assert.NotContains(t, dirEntriesBases(des), fileName)
	})

	t.Run("fails when file does not exist", func(t *testing.T) {
		fileName := "testRemove_" + randomString(10) + ".txt"
		obj := &Object{common{
			c:      c,
			remote: fileName,
		}}
		err := obj.Remove(context.TODO())
		assert.Error(t, err)
	})

	// TODO: what happens if object is directory

}

func testOpen(t *testing.T, c *Client) {
	obj, err := c.NewObject(context.TODO(), pre_existing_file_name)
	assert.NoError(t, err)
	r, err := obj.Open(context.TODO(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	bs, err := io.ReadAll(r)
	assert.NoError(t, err)
	assert.Equal(t, pre_existing_file_contents, string(bs))

}

func testUpdate(t *testing.T, c *Client) {
	fileName := "testUpdate_" + randomString(10) + ".txt"
	r, src := randomPuttableObject(fileName)
	obj, err := c.Put(context.TODO(), r, src, nil)
	assert.NoError(t, err, "was there an error while putting file to create initial test file")
	updateContent, _ := randomPuttableObject(fileName)
	updatedBytes, _ := io.ReadAll(updateContent)
	err = obj.Update(context.TODO(), bytes.NewReader(updatedBytes), src, nil)
	assert.NoError(t, err, "was there an error while updating file")
	actualContentReader, err := obj.Open(context.TODO(), nil)
	assert.NoError(t, err, "was there an error while opening updated file")
	actualBytes, err := io.ReadAll(actualContentReader)
	assert.NoError(t, err, "was there an error while reading contents of opened file")
	assert.Equal(t, actualBytes, updatedBytes)

	t.Run("is md5 correctly updated", func(t *testing.T) {
		expectedMd5 := md5.Sum(updatedBytes)
		expectedMd5Slice := expectedMd5[:]
		o, err := c.NewObject(context.TODO(), fileName)
		assert.NoError(t, err, "creating object for update file to fetch hash")
		obj := o.(*Object)
		resp, err := obj.fileClient().GetProperties(context.TODO(), nil)
		assert.NoError(t, err, "getting properties")
		assert.Equal(t, expectedMd5Slice, resp.ContentMD5)
	})

}

func dirEntriesBases(des fs.DirEntries) []string {
	bases := []string{}
	for _, de := range des {
		bases = append(bases, filepath.Base(de.Remote()))
	}
	return bases
}

// func testWalkAll(t *testing.T, c *Client) {
// 	objs, dirs, err := walk.GetAll(context.TODO(), c, "", true, -1)
// 	// walk.Walk()
// 	assert.NoError(t, err)
// 	assert.Len(t, objs, 0)
// 	assert.Len(t, dirs, 1)
// }
