package mapping_test

import (
	"os"
	"path"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/project-flotta/flotta-device-worker/internal/ansible/mapping"
)

var _ = Describe("Mapping", func() {
	const (
		peTestName        = "pe-test-name"
		peStatusDeploying = "Deploying"
		peStatusRunning   = "Running"
	)
	var configDir, sha256Test, filePathTest string
	var repo mapping.MappingRepository

	BeforeEach(func() {
		dir, err := os.MkdirTemp(os.TempDir(), "AAA")
		Expect(err).ToNot(HaveOccurred())
		configDir = dir

		repo, err = mapping.NewMappingRepository(dir)
		Expect(err).ToNot(HaveOccurred())

		sha256Test = repo.GetSha256([]byte("test"))
		filePathTest = path.Join(configDir, sha256Test)
	})
	AfterEach(func() {
		err := repo.RemoveMappingFile()
		Expect(err).ToNot(HaveOccurred())
	})
	It("sha256 Generation", func() {
		s1 := repo.GetSha256([]byte("AAA"))
		s2 := repo.GetSha256([]byte("AAA"))
		Expect(s1).To(Equal(s2))
	})
	It("Should be created empty", func() {
		// then
		Expect(repo).ToNot(BeNil())
		Expect(repo.Size()).To(BeZero())
	})

	It("Should return timeZero for non-existing file", func() {
		// when
		modTime := repo.GetModTime("not-here")

		// then
		Expect(modTime).To(Equal(int64(0)))
	})

	It("Should return empty string for non-existing modTime", func() {
		// when
		name := repo.GetFilePath(time.Now())

		// then
		Expect(name).To(BeEmpty())
	})

	It("Should store and return values", func() {
		// when
		modTime := time.Now()
		err := repo.Add(peTestName, []byte("test"), modTime, peStatusDeploying)

		// then
		Expect(err).ToNot(HaveOccurred())

		Expect(repo.GetModTime(filePathTest)).To(Equal(modTime.UnixNano()))
		Expect(repo.GetFilePath(modTime)).To(Equal(path.Join(configDir, sha256Test)))
		Expect(repo.GetName(filePathTest)).To(Equal(peTestName))
		Expect(repo.GetStatus(peTestName)).To(Equal(peStatusDeploying))
	})

	It("Should remove mapping", func() {
		// given
		modTime := time.Now()
		err := repo.Add(peTestName, []byte("test"), modTime, peStatusDeploying)
		Expect(err).ToNot(HaveOccurred())

		// when
		err = repo.Remove([]byte("test"))

		// then
		Expect(err).ToNot(HaveOccurred())
		Expect(repo.GetModTime(filePathTest)).To(Equal(int64(0)))
		Expect(repo.GetFilePath(modTime)).To(BeEmpty())
		Expect(repo.GetName(filePathTest)).To(BeEmpty())
		Expect(repo.GetStatus(peTestName)).To(Equal(peStatusDeploying))

	})

	It("Should persist mappings", func() {
		// given
		filePath1 := path.Join(configDir, repo.GetSha256([]byte("test-one")))
		filePath2 := path.Join(configDir, repo.GetSha256([]byte("test-two")))
		modTime1 := time.Now()
		modTime2 := modTime1.Add(1 * time.Minute)
		peTestName1 := peTestName + "1"
		err := repo.Add(peTestName1, []byte("test-one"), modTime1, peStatusDeploying)
		Expect(err).ToNot(HaveOccurred())
		Expect(repo.GetModTime(filePath1)).To(Equal(modTime1.UnixNano()))
		Expect(repo.GetFilePath(modTime1)).To(Equal(filePath1))
		Expect(repo.GetName(filePath1)).To(Equal(peTestName1))
		Expect(repo.GetStatus(peTestName1)).To(Equal(peStatusDeploying))

		peTestName2 := peTestName + "2"

		err = repo.Add(peTestName2, []byte("test-two"), modTime2, peStatusRunning)
		Expect(err).ToNot(HaveOccurred())
		// when
		repo2, err := mapping.NewMappingRepository(configDir)

		// then
		Expect(err).ToNot(HaveOccurred())
		Expect(repo2.GetModTime(filePath1)).To(Equal(modTime1.UnixNano()))
		Expect(repo2.GetFilePath(modTime1)).To(Equal(filePath1))
		Expect(repo2.GetName(filePath1)).To(Equal(peTestName1))
		Expect(repo2.GetStatus(peTestName1)).To(Equal(peStatusDeploying))

		Expect(repo2.GetModTime(filePath2)).To(Equal(modTime2.UnixNano()))
		Expect(repo2.GetFilePath(modTime2)).To(Equal(filePath2))
		Expect(repo2.GetName(filePath2)).To(Equal(peTestName2))
		Expect(repo2.GetStatus(peTestName2)).To(Equal(peStatusRunning))

	})

})
