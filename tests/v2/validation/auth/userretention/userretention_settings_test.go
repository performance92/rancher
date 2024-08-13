//go:build (validation || infra.any || cluster.any || sanity) && !stress && !extended

package userretention

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

type UserRetentionSettingsTestSuite struct {
	suite.Suite
	client  *rancher.Client
	session *session.Session
}

func (ur *UserRetentionSettingsTestSuite) SetupSuite() {
	ur.session = session.NewSession()
	client, err := rancher.NewClient("", ur.session)
	require.NoError(ur.T(), err)
	ur.client = client
}

func (ur *UserRetentionSettingsTestSuite) TearDownSuite() {
	ur.session.Cleanup()
}

func (ur *UserRetentionSettingsTestSuite) testPositiveInputValues(settingName string, tests []struct {
	defaultValue string
	value        string
	description  string
}) {
	logrus.Infof("Updating %s settings with positive values:", settingName)
	for _, test := range tests {
		ur.T().Run(test.description, func(*testing.T) {
			err := updateUserRetentionSettings(ur.client, settingName, test.defaultValue, test.value)
			assert.NoError(ur.T(), err, "Unexpected error updating settings")

			if err == nil {
				settings, err := ur.client.Management.Setting.ByID(settingName)
				assert.NoError(ur.T(), err, "Failed to retrieve settings")

				if err == nil {
					assert.Equal(ur.T(), settingName, settings.Name, "Setting name should match")

					expectedValue := test.value
					if expectedValue == "" {
						expectedValue = test.defaultValue
					}
					assert.Equal(ur.T(), expectedValue, settings.Value, "Setting value should match expected")

					if test.defaultValue != "" {
						assert.Equal(ur.T(), test.defaultValue, settings.Default, "Default value should match when provided")
					} else {
						assert.Empty(ur.T(), settings.Default, "Default value should be empty when not provided")
					}

					logrus.Infof("Setting updated - Name: %s, Default: %s, Value: %s",
						settings.Name, settings.Default, settings.Value)
				}
			}

			logrus.Infof("Test completed: %s", test.description)
		})
	}
}
func (ur *UserRetentionSettingsTestSuite) testNegativeInputValues(settingName string, tests []struct {
	defaultValue string
	value        string
	description  string
}) {
	logrus.Infof("Updating %s settings with negative values:", settingName)
	for _, test := range tests {
		ur.T().Run(test.description, func(*testing.T) {

			initialSettings, getErr := ur.client.Management.Setting.ByID(settingName)
			assert.NoError(ur.T(), getErr, "Failed to retrieve initial settings")

			err := updateUserRetentionSettings(ur.client, settingName, test.defaultValue, test.value)
			assert.Error(ur.T(), err, "Expected an error for input '%s', but got nil", test.value)

			if err != nil {
				ur.validateError(err, test.description)

				updatedSettings, getErr := ur.client.Management.Setting.ByID(settingName)
				assert.NoError(ur.T(), getErr, "Failed to retrieve updated settings")

				if getErr == nil {
					assert.NotEqual(ur.T(), test.value, updatedSettings.Value, "Value should not be updated for invalid input")

					if initialSettings.Default != updatedSettings.Default {
						logrus.Warnf("Default value changed from %s to %s for invalid input", initialSettings.Default, updatedSettings.Default)
					}

					assert.NotEqual(ur.T(), test.value, updatedSettings.Value, "Value should not be updated to invalid input")
				}

				logrus.Infof("Failed to update %s settings as expected. Error: %v", settingName, err)
			}
		})
	}
}

func (ur *UserRetentionSettingsTestSuite) validateError(err error, expectedDescription string) {
	var statusErr *apierrors.StatusError
	var found bool

	switch e := err.(type) {
	case interface{ Unwrap() error }:
		if innerErr := e.Unwrap(); innerErr != nil {
			if innerWrapper, ok := innerErr.(interface{ Unwrap() error }); ok {
				if deepestErr := innerWrapper.Unwrap(); deepestErr != nil {
					statusErr, found = deepestErr.(*apierrors.StatusError)
				}
			}
		}
	}

	if found && statusErr != nil {
		assert.Equal(ur.T(), int32(400), statusErr.ErrStatus.Code, "Status code should be 400")
		assert.Equal(ur.T(), metav1.StatusReasonBadRequest, statusErr.ErrStatus.Reason, "Reason should be BadRequest")
		assert.Contains(ur.T(), statusErr.ErrStatus.Message, expectedDescription, "Error should contain the expected description")
	} else {
		errMsg := err.Error()
		assert.Contains(ur.T(), errMsg, "denied the request", "Error should mention denied request")
		assert.Contains(ur.T(), errMsg, expectedDescription, "Error should contain the expected description")
	}
}

func (ur *UserRetentionSettingsTestSuite) validateSettingsNotUpdated(settingName, inputValue string) {
	settings, err := ur.client.Management.Setting.ByID(settingName)
	if assert.NoError(ur.T(), err, "Failed to retrieve settings") {
		assert.Equal(ur.T(), settingName, settings.Name)
		assert.NotEqual(ur.T(), defaultSettingValue, settings.Default)
		assert.NotEqual(ur.T(), inputValue, settings.Value)
	}
}

func (ur *UserRetentionSettingsTestSuite) TestUpdateSettingsForDisableUserWithPositiveInputValues() {
	tests := []struct {
		defaultValue string
		value        string
		description  string
	}{
		{"", "", "No action - users will not be deactivated"},
		{"", "0s", "Users will be deactivated after 0s"},
		{"", "0m", "Users will be deactivated after 0m"},
		{"", "0h", "Users will be deactivated after 0h"},
		{"", "10s", "Users will be deactivated after 10s"},
		{"", "10m", "Users will be deactivated after 10m"},
		{"", "20h", "Users will be deactivated after 20h"},
		{"", "10000s", "Users will be deactivated after 10000s"},
		{"", "10000m", "Users will be deactivated after 10000m"},
		{"", "10000h", "Users will be deactivated after 10000h"},
		{"", "", "No action - users will not be deactivated"},
		{"0s", "", "Users will be deactivated after 0s"},
		{"0m", "", "Users will be deactivated after 0m"},
		{"0h", "", "Users will be deactivated after 0h"},
		{"10s", "", "Users will be deactivated after 10s"},
		{"10m", "", "Users will be deactivated after 10m"},
		{"20h", "", "Users will be deactivated after 20h"},
		{"10000s", "", "Users will be deactivated after 10000s"},
		{"10000m", "", "Users will be deactivated after 10000m"},
		{"10000h", "", "Users will be deactivated after 10000h"},
		{"20h", "20m", "Users will be deactivated after 20m"},
		{"20m", "20h", "Users will be deactivated after 20h"},
	}
	setupUserRetentionSettings(ur.client, "", "", "", "")
	ur.testPositiveInputValues(disableInactiveUserAfter, tests)
}

func (ur *UserRetentionSettingsTestSuite) TestUpdateSettingsForDisableInactiveUserAfterWithNegativeInputValues() {
	tests := []struct {
		defaultValue string
		value        string
		description  string
	}{
		{"", "10", "Invalid value: \"10\": time: missing unit in duration \"10\""},
		{"", "10S", "Invalid value: \"10S\": time: unknown unit \"S\" in duration \"10S\""},
		{"", "10M", "Invalid value: \"10M\": time: unknown unit \"M\" in duration \"10M\""},
		{"", "10H", "Invalid value: \"10H\": time: unknown unit \"H\" in duration \"10H\""},
		{"", "10sec", "Invalid value: \"10sec\": time: unknown unit \"sec\" in duration \"10sec\""},
		{"", "10min", "Invalid value: \"10min\": time: unknown unit \"min\" in duration \"10min\""},
		{"", "20hour", "Invalid value: \"20hour\": time: unknown unit \"hour\" in duration \"20hour\""},
		{"", "1d", "Invalid value: \"1d\": time: unknown unit \"d\" in duration \"1d\""},
		{"", "-20m", "Invalid value: \"-20m\": negative duration"},
		{"", "tens", "Invalid value: \"tens\": time: invalid duration \"tens\""},
		// {"10", "", "Invalid value: \"10\": time: missing unit in duration \"10\""},
		// {"10S", "", "Invalid value: \"10S\": time: unknown unit \"S\" in duration \"10S\""},
		// {"10M", "", "Invalid value: \"10M\": time: unknown unit \"M\" in duration \"10M\""},
		// {"10H", "", "Invalid value: \"10H\": time: unknown unit \"H\" in duration \"10H\""},
		// {"10sec", "", "Invalid value: \"10sec\": time: unknown unit \"sec\" in duration \"10sec\""},
		// {"10min", "", "Invalid value: \"10min\": time: unknown unit \"min\" in duration \"10min\""},
		// {"20hour", "", "Invalid value: \"20hour\": time: unknown unit \"hour\" in duration \"20hour\""},
		// {"1d", "", "Invalid value: \"1d\": time: unknown unit \"d\" in duration \"1d\""},
		// {"-20m", "", "Invalid value: \"-20m\": negative duration"},
		// {"tens", "", "Invalid value: \"tens\": time: invalid duration \"tens\""},
		// {"-20m", "1d", "Invalid value: \"-20m\": negative duration"},
		// {"", "-20m", "Invalid value: \"-20m\": negative duration"},
		// {"1d", "-20m", "Invalid value: \"1d\": time: unknown unit \"d\" in duration \"1d\""},
	}
	ur.testNegativeInputValues(disableInactiveUserAfter, tests)
	setupUserRetentionSettings(ur.client, "", "", "", "false")
}

func (ur *UserRetentionSettingsTestSuite) TestUpdateSettingsForDeleteInactiveUserAfterWithPositiveInputValues() {
	tests := []struct {
		defaultValue string
		value        string
		description  string
	}{
		{"", "", "No action - users will not be deleted"},
		{"", "100000000s", "Users will delete after 100000000s"},
		{"", "200000m", "Users will delete after 200000m"},
		{"", "10000h", "Users will delete after 10000h"},
		{"100000000s", "", "Users will delete after 100000000s"},
		{"200000m", "", "Users will delete after 200000m"},
		{"10000h", "", "Users will delete after 10000h"},
		{"10000h", "200000m", "Users will delete after 200000m"},
		{"200000m", "10000h", "Users will delete after 10000h"},
	}
	ur.testPositiveInputValues(deleteInactiveUserAfter, tests)
	setupUserRetentionSettings(ur.client, "", "", "", "false")
}

func (ur *UserRetentionSettingsTestSuite) TestUpdateSettingsForDeleteInactiveUserAfterWithNegativeInputValues() {
	tests := []struct {
		defaultValue string
		value        string
		description  string
	}{
		{"", "10", "Invalid value: \"10\": time: missing unit in duration \"10\""},
		{"", "10s", "Invalid value: \"10s\": must be at least 336h0m0s"},
		{"", "10m", "Invalid value: \"10m\": must be at least 336h0m0s"},
		{"", "10h", "Invalid value: \"10h\": must be at least 336h0m0s"},
		{"", "10S", "Invalid value: \"10S\": time: unknown unit \"S\" in duration \"10S\""},
		{"", "10M", "Invalid value: \"10M\": time: unknown unit \"M\" in duration \"10M\""},
		{"", "10H", "Invalid value: \"10H\": time: unknown unit \"H\" in duration \"10H\""},
		{"", "10sec", "Invalid value: \"10sec\": time: unknown unit \"sec\" in duration \"10sec\""},
		{"", "10min", "Invalid value: \"10min\": time: unknown unit \"min\" in duration \"10min\""},
		{"", "20hour", "Invalid value: \"20hour\": time: unknown unit \"hour\" in duration \"20hour\""},
		{"", "1d", "Invalid value: \"1d\": time: unknown unit \"d\" in duration \"1d\""},
		{"", "-20m", "Invalid value: \"-20m\": negative duration"},
		// {"10", "", "Invalid value: \"10\": time: missing unit in duration \"10\""},
		// {"10s", "", "Invalid value: \"10s\": must be at least 336h0m0s"},
		// {"10m", "", "Invalid value: \"10m\": must be at least 336h0m0s"},
		// {"10h", "", "Invalid value: \"10h\": must be at least 336h0m0s"},
		// {"10S", "", "Invalid value: \"10S\": time: unknown unit \"S\" in duration \"10S\""},
		// {"10M", "", "Invalid value: \"10M\": time: unknown unit \"M\" in duration \"10M\""},
		// {"10H", "", "Invalid value: \"10H\": time: unknown unit \"H\" in duration \"10H\""},
		// {"10sec", "", "Invalid value: \"10sec\": time: unknown unit \"sec\" in duration \"10sec\""},
		// {"10min", "", "Invalid value: \"10min\": time: unknown unit \"min\" in duration \"10min\""},
		// {"20hour", "", "Invalid value: \"20hour\": time: unknown unit \"hour\" in duration \"20hour\""},
		// {"1d", "", "Invalid value: \"1d\": time: unknown unit \"d\" in duration \"1d\""},
		// {"-20m", "", "Invalid value: \"-20m\": negative duration"},
		// {"-20m", "10000h", "Invalid value: \"-20m\": negative duration"},
		// {"10000h", "-20m", "Invalid value: \"-20m\": negative duration"},
		// {"-20m", "20m", "Invalid value: \"-20m\": negative duration"},
	}
	ur.testNegativeInputValues(deleteInactiveUserAfter, tests)
	setupUserRetentionSettings(ur.client, "", "", "", "false")
}

func (ur *UserRetentionSettingsTestSuite) TestUpdateSettingsForUserRetentionCronWithPositiveInputValues() {
	tests := []struct {
		defaultValue string
		value        string
		description  string
	}{
		{"", "0 * * * *", "every 1 hour"},
		{"", "0 0 * * *", "every 1 day"},
		{"", "*/5 * * * *", "every 5 mins"},
		{"", "*/1 * * * *", "every min"},
		{"", "* * * * *", "every min"},
		{"", "30/1 * * * *", "every 30 sec"},
		{"", "0-5 14 * * *", "every minute starting at 2:00 PM and ending at 2:05 PM, every day"},
		{"", "0 0 1,2 * *", "at midnight of 1st, 2nd day of each month"},
		{"", "0 0 1,2 * 3", "at midnight of 1st, 2nd day of each month, and each Wednesday"},
		{"0 * * * *", "", "every 1 hour"},
		{"0 0 * * *", "", "every 1 day"},
		{"*/5 * * * *", "", "every 5 mins"},
		{"*/1 * * * *", "", "every min"},
		{"* * * * *", "", "every min"},
		{"30/1 * * * *", "", "every 30 sec"},
		{"0-5 14 * * *", "", "every minute starting at 2:00 PM and ending at 2:05 PM, every day"},
		{"0 0 1,2 * *", "", "at midnight of 1st, 2nd day of each month"},
		{"0 0 1,2 * 3", "", "at midnight of 1st, 2nd day of each month, and each Wednesday"},
		{"* * * * *", "30/1 * * * *", "every 30 sec"},
		{"30/1 * * * *", "* * * * *", "every min"},
	}
	ur.testPositiveInputValues(userRetentionCron, tests)
	setupUserRetentionSettings(ur.client, "", "", "", "false")
}

func (ur *UserRetentionSettingsTestSuite) TestUpdateSettingsForUserRetentionCronWithNegativeInputValues() {
	tests := []struct {
		defaultValue string
		value        string
		description  string
	}{
		{"", "* * * * * *", "Invalid value: \"* * * * * *\": Expected exactly 5 fields, found 6: * * * * * *"},
		{"", "*/-1 * * * *", "Invalid value: \"*/-1 * * * *\": Negative number (-1) not allowed: -1"},
		{"", "60/1 * * * *", "Invalid value: \"60/1 * * * *\": Beginning of range (60) beyond end of range (59): 60/1"},
		{"", "-30/1 * * * *", "Invalid value: \"-30/1 * * * *\": Failed to parse int from : strconv.Atoi: parsing \"\": invalid syntax"},
		{"", "(*/1) * * * *", "Invalid value: \"(*/1) * * * *\": Failed to parse int from (*: strconv.Atoi: parsing \"(*\": invalid syntax"},
		{"", "10min", "Invalid value: \"10min\": Expected exactly 5 fields, found 1: 10min"},
		{"", "* * * * * */2", "Invalid value: \"* * * * * */2\": Expected exactly 5 fields, found 6: * * * * * */2"},
		{"", "1d", "Invalid value: \"1d\": Expected exactly 5 fields, found 1: 1d"},
		{"", "-20m", "Invalid value: \"-20m\": Expected exactly 5 fields, found 1: -20m"},
		// {"* * * * * *", "", "Invalid value: \"* * * * * *\": Expected exactly 5 fields, found 6: * * * * * *"},
		// {"*/-1 * * * *", "", "Invalid value: \"*/-1 * * * *\": Negative number (-1) not allowed: -1"},
		// {"60/1 * * * *", "", "Invalid value: \"60/1 * * * *\": Beginning of range (60) beyond end of range (59): 60/1"},
		// {"-30/1 * * * *", "", "Invalid value: \"-30/1 * * * *\": Failed to parse int from : strconv.Atoi: parsing \"\": invalid syntax"},
		// {"(*/1) * * * *", "", "Invalid value: \"(*/1) * * * *\": Failed to parse int from (*: strconv.Atoi: parsing \"(*\": invalid syntax"},
		// {"10min", "", "Invalid value: \"10min\": Expected exactly 5 fields, found 1: 10min"},
		// {"* * * * * */2", "", "Invalid value: \"* * * * * */2\": Expected exactly 5 fields, found 6: * * * * * */2"},
		// {"1d", "", "Invalid value: \"1d\": Expected exactly 5 fields, found 1: 1d"},
		// {"-20m", "", "Invalid value: \"-20m\": Expected exactly 5 fields, found 1: -20m"},
		// {"1d", "1d", "Invalid value: \"1d\": Expected exactly 5 fields, found 1: 1d"},
		// {"0 * * * *", "", "Invalid value: \"1d\": Expected exactly 5 fields, found 1: 1d"},
		// {"", "0 * * * *", "Invalid value: \"1d\": Expected exactly 5 fields, found 1: 1d"},
	}
	ur.testNegativeInputValues(userRetentionCron, tests)
	setupUserRetentionSettings(ur.client, "", "", "", "false")
}

func TestUserRetentionSettingsSuite(t *testing.T) {
	suite.Run(t, new(UserRetentionSettingsTestSuite))
}
