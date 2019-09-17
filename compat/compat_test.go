package compat

import (
	"bytes"
	bplog "github.com/buildpack/libbuildpack/logger"
	"github.com/cloudfoundry/libcfbuildpack/logger"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudfoundry/libcfbuildpack/buildpackplan"
	"github.com/cloudfoundry/libcfbuildpack/helper"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"

	"github.com/cloudfoundry/libcfbuildpack/test"
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"
)

func TestUnitCompat(t *testing.T) {
	spec.Run(t, "Compat", testCompat, spec.Report(report.Terminal{}))
}

func testCompat(t *testing.T, when spec.G, it spec.S) {
	var factory *test.BuildFactory
	var appRoot string

	it.Before(func() {
		RegisterTestingT(t)
		factory = test.NewBuildFactory(t)
		factory.AddPlan(buildpackplan.Plan{Name: Layer})
		appRoot = factory.Build.Application.Root
	})

	when("an options.json exists", func() {
		it.Before(func() {
			json := `{
				"WEB_SERVER": "httpd",
				"HTTPD_VERSION": "2.4.39",
				"PHP_VERSION": "7.3.10",
				"NGINX_VERSION": "1.14.3",
				"COMPOSER_VERSION": "1.9.0",
				"ADDITIONAL_PREPROCESS_CMDS": ["some-command", "another-command"],
				"COMPOSER_INSTALL_GLOBAL": ["global1", "global2", "global3"],
				"COMPOSER_INSTALL_OPTIONS": ["install1", "install2", "install3"],
				"COMPOSER_VENDOR_DIR": "vendor"}`
			err := writeOptionsJSON(appRoot, json)
			Expect(err).ToNot(HaveOccurred())
		})

		it.After(func() {
			os.RemoveAll(filepath.Join(appRoot, ".bp-config"))
		})

		when("and we're loading options", func() {
			it("can load PHP options", func() {
				options, err := LoadOptionsJSON(appRoot)
				Expect(err).ToNot(HaveOccurred())
				Expect(options.PHP.WebServer).To(Equal("httpd"))
				Expect(options.PHP.Version).To(Equal("7.3.10"))
			})
			it("can load HTTPD options", func() {
				options, err := LoadOptionsJSON(appRoot)
				Expect(err).ToNot(HaveOccurred())
				Expect(options.HTTPD.Version).To(Equal("2.4.39"))
			})
			it("can load Nginx options", func() {
				options, err := LoadOptionsJSON(appRoot)
				Expect(err).ToNot(HaveOccurred())
				Expect(options.Nginx.Version).To(Equal("1.14.3"))
			})
			it("can load Composer options", func() {
				options, err := LoadOptionsJSON(appRoot)
				Expect(err).ToNot(HaveOccurred())
				Expect(options.Composer.Version).To(Equal("1.9.0"))
				Expect(options.Composer.GlobalOptions).To(ConsistOf("global1", "global2", "global3"))
				Expect(options.Composer.InstallOptions).To(ConsistOf("install1", "install2", "install3"))
				Expect(options.Composer.VendorDirectory).To(Equal("vendor"))
			})
		})

		when("and contains additional commands", func() {
			it("will copy those to a `.profile.d` script", func() {
				contributor, _, err := NewContributor(factory.Build)
				Expect(err).ToNot(HaveOccurred())
				options, err := LoadOptionsJSON(appRoot)
				Expect(err).ToNot(HaveOccurred())
				contributor.MigrateAdditionalCommands(options)
				pathToAdditionalCMDS := filepath.Join(appRoot, ".profile.d", "additional-cmds.sh")

				Expect(pathToAdditionalCMDS).To(BeARegularFile())
				additionalCMDS, err := ioutil.ReadFile(pathToAdditionalCMDS)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(additionalCMDS)).To(Equal("some-command\nanother-command\n"))
			})
		})
	})

	when("options.json exists and there are specific version requirements", func() {
		it("loads PHP_DEFAULT", func() {
			json := `{"PHP_VERSION": "{PHP_DEFAULT}"}`
			err := writeOptionsJSON(appRoot, json)
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(filepath.Join(appRoot, ".bp-config"))

			options, err := LoadOptionsJSON(appRoot)
			Expect(err).ToNot(HaveOccurred())
			Expect(options.PHP.Version).To(BeEmpty())
		})
		it("loads PHP_71_LATEST", func() {
			json := `{"PHP_VERSION": "{PHP_71_LATEST}"}`
			err := writeOptionsJSON(appRoot, json)
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(filepath.Join(appRoot, ".bp-config"))

			options, err := LoadOptionsJSON(appRoot)
			Expect(err).ToNot(HaveOccurred())
			Expect(options.PHP.Version).To(Equal("7.1.*"))
		})
		it("loads PHP_72_LATEST", func() {
			json := `{"PHP_VERSION": "{PHP_72_LATEST}"}`
			err := writeOptionsJSON(appRoot, json)
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(filepath.Join(appRoot, ".bp-config"))

			options, err := LoadOptionsJSON(appRoot)
			Expect(err).ToNot(HaveOccurred())
			Expect(options.PHP.Version).To(Equal("7.2.*"))
		})
		it("loads PHP_73_LATEST", func() {
			json := `{"PHP_VERSION": "{PHP_73_LATEST}"}`
			err := writeOptionsJSON(appRoot, json)
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(filepath.Join(appRoot, ".bp-config"))

			options, err := LoadOptionsJSON(appRoot)
			Expect(err).ToNot(HaveOccurred())
			Expect(options.PHP.Version).To(Equal("7.3.*"))
		})

	})

	when("options need to be written", func() {
		it("produces buildpack.yml", func() {
			options := Options{
				HTTPD: HTTPDOptions{
					Version: "2.3.49",
				},
				PHP: PHPOptions{
					Version:   "7.3.10",
					WebServer: "standalone",
				},
				Nginx: NginxOptions{
					Version: "1.14.9",
				},
				Composer: ComposerOptions{
					Version: "1.8.9",
					GlobalOptions: []string{},
					InstallOptions: []string{},
				},
			}
			err := WriteOptionsToBuildpackYAML(appRoot, options)
			Expect(err).ToNot(HaveOccurred())

			exists, err := helper.FileExists(filepath.Join(appRoot, "buildpack.yml"))
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeTrue())

			buildpackYMLOutput, err := ioutil.ReadFile(filepath.Join(appRoot, "buildpack.yml"))
			Expect(err).ToNot(HaveOccurred())

			actualOptions := Options{}
			err = yaml.Unmarshal(buildpackYMLOutput, &actualOptions)
			Expect(err).ToNot(HaveOccurred())

			Expect(options).To(Equal(actualOptions))
		})
	})

	when("extensions need to be migrated", func() {
		it("migrates PHP_EXTENSIONS", func() {
			c, _, err := NewContributor(factory.Build)
			Expect(err).ToNot(HaveOccurred())
			options := Options{
				PHP: PHPOptions{
					Extensions: []string{"ext1", "ext2"},
				},
			}

			err = c.MigrateExtensions(options)
			Expect(err).ToNot(HaveOccurred())

			extensionOutput, err := ioutil.ReadFile(filepath.Join(appRoot, ".php.ini.d", "compat-extensions.ini"))
			Expect(err).ToNot(HaveOccurred())

			Expect(string(extensionOutput)).To(ContainSubstring("extension=ext1.so"))
			Expect(string(extensionOutput)).To(ContainSubstring("extension=ext2.so"))
		})

		it("migrates ZEND_EXTENSIONS", func() {
			c, _, err := NewContributor(factory.Build)
			Expect(err).ToNot(HaveOccurred())
			options := Options{
				PHP: PHPOptions{
					ZendExtensions: []string{"zext1", "zext2"},
				},
			}

			err = c.MigrateExtensions(options)
			Expect(err).ToNot(HaveOccurred())

			extensionOutput, err := ioutil.ReadFile(filepath.Join(appRoot, ".php.ini.d", "compat-extensions.ini"))
			Expect(err).ToNot(HaveOccurred())

			Expect(string(extensionOutput)).To(ContainSubstring("zend_extension=zext1.so"))
			Expect(string(extensionOutput)).To(ContainSubstring("zend_extension=zext2.so"))
		})
	})

	when(".bp-config/httpd or `.bp-config/nginx` exists", func() {
		it("contains *.conf files", func() {
			c, _, err := NewContributor(factory.Build)
			Expect(err).ToNot(HaveOccurred())

			err = helper.WriteFile(filepath.Join(appRoot, ".bp-config", "httpd", "test.conf"), 0644, "contents")
			Expect(err).ToNot(HaveOccurred())
			err = helper.WriteFile(filepath.Join(appRoot, ".bp-config", "httpd", "anoter.conf"), 0644, "more contents")
			Expect(err).ToNot(HaveOccurred())

			err = c.ErrorOnCustomServerConfig("HTTPD", "httpd", ".conf")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("migration failure"))
		})

		it("contains *.conf files", func() {
			c, _, err := NewContributor(factory.Build)
			Expect(err).ToNot(HaveOccurred())

			err = helper.WriteFile(filepath.Join(appRoot, ".bp-config", "nginx", "test.conf"), 0644, "contents")
			Expect(err).ToNot(HaveOccurred())
			err = helper.WriteFile(filepath.Join(appRoot, ".bp-config", "nginx", "anoter.conf"), 0644, "more contents")
			Expect(err).ToNot(HaveOccurred())

			err = c.ErrorOnCustomServerConfig("Nginx", "nginx", ".conf")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("migration failure"))
		})

		it("doesn't contain *.conf files", func() {
			c, _, err := NewContributor(factory.Build)
			Expect(err).ToNot(HaveOccurred())

			err = helper.WriteFile(filepath.Join(appRoot, ".bp-config", "httpd", "test.txt"), 0644, "contents")
			Expect(err).ToNot(HaveOccurred())

			err = c.ErrorOnCustomServerConfig("HTTPD", "httpd", ".conf")
			Expect(err).ToNot(HaveOccurred())
		})
	})

	when(".bp-config/php/ exists", func() {
		it("subfolder php.ini.d contains *.ini files", func() {
			c, _, err := NewContributor(factory.Build)
			Expect(err).ToNot(HaveOccurred())

			err = helper.WriteFile(filepath.Join(appRoot, ".bp-config", "php", "php.ini.d", "test.ini"), 0644, "contents")
			Expect(err).ToNot(HaveOccurred())
			err = helper.WriteFile(filepath.Join(appRoot, ".bp-config", "php", "php.ini.d", "another.ini"), 0644, "more contents")
			Expect(err).ToNot(HaveOccurred())

			err = c.MigratePHPSnippets("PHP INI", "php.ini.d", ".php.ini.d", "ini")

			Expect(err).ToNot(HaveOccurred())

			Expect(filepath.Join(appRoot, ".php.ini.d", "test.ini")).To(BeARegularFile())
			Expect(filepath.Join(appRoot, ".php.ini.d", "another.ini")).To(BeARegularFile())
		})

		it("subfolder fpm.d contains *.conf files", func() {
			c, _, err := NewContributor(factory.Build)
			Expect(err).ToNot(HaveOccurred())

			err = helper.WriteFile(filepath.Join(appRoot, ".bp-config", "php", "fpm.d", "test.conf"), 0644, "contents")
			Expect(err).ToNot(HaveOccurred())
			err = helper.WriteFile(filepath.Join(appRoot, ".bp-config", "php", "fpm.d", "another.conf"), 0644, "more contents")
			Expect(err).ToNot(HaveOccurred())

			err = c.MigratePHPSnippets("PHP-FPM", "fpm.d", ".php.fpm.d", "conf")
			Expect(err).ToNot(HaveOccurred())

			Expect(filepath.Join(appRoot, ".php.fpm.d", "test.conf")).To(BeARegularFile())
			Expect(filepath.Join(appRoot, ".php.fpm.d", "another.conf")).To(BeARegularFile())
		})
	})

	when("a composer.json file exists", func() {
		it("logs a warning that we no longer move vendor", func() {
			buf := bytes.Buffer{}
			info := logger.Logger{
				Logger: bplog.NewLogger(&buf, &buf),
			}
			factory.Build.Logger = info

			c, _, err := NewContributor(factory.Build)
			Expect(err).ToNot(HaveOccurred())

			err = helper.WriteFile(filepath.Join(appRoot, "composer.json"), 0644, "contents")
			Expect(err).ToNot(HaveOccurred())

			err = c.Contribute()
			Expect(err).ToNot(HaveOccurred())

			Expect(buf.String()).To(ContainSubstring("The vendor directory is no longer migrated to LIBDIR."))
		})
	})

	when("we have a web app", func() {
		when("and no WEBDIR is set", func() {
			it("does nothing", func() {
				filesToMake := []string{
					"composer.json",
					".extensions/something/somefile.py",
					"lib/test.php",
					".profile",
					"htdocs/other/files/app.php",
					"htdocs/index.php",
					"htdocs/more.php",
				}

				for _, fileToMake := range filesToMake {
					err := helper.WriteFile(filepath.Join(appRoot, fileToMake), 0644, "contents")
					Expect(err).ToNot(HaveOccurred())
				}

				c, _, err := NewContributor(factory.Build)
				Expect(err).ToNot(HaveOccurred())

				options := Options{
					PHP:      PHPOptions{
						LibDir: "lib",
					},
				}
				err = c.MoveWebFilesToWebDir(options)
				Expect(err).ToNot(HaveOccurred())

				for _, fileToNotMove := range filesToMake {
					Expect(filepath.Join(appRoot, fileToNotMove)).To(BeARegularFile())
				}
			})
		})

		when("and WEBDIR exists", func() {
			it("does nothing", func() {
				filesToMake := []string{
					"composer.json",
					".extensions/something/somefile.py",
					"lib/test.php",
					".profile",
					"htdocs/other/files/app.php",
					"htdocs/index.php",
					"htdocs/more.php",
				}

				for _, fileToMake := range filesToMake {
					err := helper.WriteFile(filepath.Join(appRoot, fileToMake), 0644, "contents")
					Expect(err).ToNot(HaveOccurred())
				}

				c, _, err := NewContributor(factory.Build)
				Expect(err).ToNot(HaveOccurred())

				options := Options{
					PHP:      PHPOptions{
						WebDir: "htdocs",
						LibDir: "lib",
					},
				}
				err = c.MoveWebFilesToWebDir(options)
				Expect(err).ToNot(HaveOccurred())

				for _, fileToNotMove := range filesToMake {
					Expect(filepath.Join(appRoot, fileToNotMove)).To(BeARegularFile())
					Expect(filepath.Join(appRoot, options.PHP.WebDir, fileToNotMove)).ToNot(BeARegularFile())
				}
			})
		})

		when("and WEBDIR doesn't exist", func() {
			it("creates WEBDIR and moves files into WEBDIR", func() {
				filesToMake := []string{
					"composer.json",
					"index.php",
					".extensions/something/somefile.py",
					"lib/test.php",
					".profile",
					"more.php",
					"other/files/app.php",
				}

				for _, fileToMake := range filesToMake {
					err := helper.WriteFile(filepath.Join(appRoot, fileToMake), 0644, "contents")
					Expect(err).ToNot(HaveOccurred())
				}

				c, _, err := NewContributor(factory.Build)
				Expect(err).ToNot(HaveOccurred())

				options := Options{
					PHP:      PHPOptions{
						WebDir: "htdocs",
						LibDir: "lib",
					},
				}
				err = c.MoveWebFilesToWebDir(options)
				Expect(err).ToNot(HaveOccurred())

				moveList := []string {
					"composer.json",
					"index.php",
					"more.php",
					"other/files/app.php",
				}
				for _, fileToMove := range moveList {
					Expect(filepath.Join(appRoot, fileToMove)).ToNot(BeARegularFile())
					Expect(filepath.Join(appRoot, options.PHP.WebDir, fileToMove)).To(BeARegularFile())
				}

				notMovedList := []string{
					".extensions/something/somefile.py",
					"lib/test.php",
					".profile",
				}
				for _, fileToNotMove := range notMovedList {
					Expect(filepath.Join(appRoot, fileToNotMove)).To(BeARegularFile())
					Expect(filepath.Join(appRoot, options.PHP.WebDir, fileToNotMove)).ToNot(BeARegularFile())
				}

				Expect(filepath.Join(appRoot, ".extensions")).To(BeADirectory())
				Expect(filepath.Join(appRoot, "other")).ToNot(BeADirectory())
			})
		})
	})
}

func writeOptionsJSON(appRoot, jsonBody string) error {
	optionsJson := filepath.Join(appRoot, ".bp-config", "options.json")
	err := helper.WriteFile(optionsJson, 0655, "%s", jsonBody)
	if err != nil {
		return err
	}
	return nil
}
