package common

import (
	"errors"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	wranglerfake "github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

func TestSetPrincipalOnCurrentUserByUserID(t *testing.T) {
	testCases := []struct {
		name             string
		userID           string
		principal        v3.Principal
		existingUser     *v3.User
		existingError    error
		principalUser    *v3.User
		principalError   error
		expectedUser     *v3.User
		expectedError    error
		expectedToUpdate bool
	}{
		{
			name:   "successfully add principal to user",
			userID: "user1",
			principal: v3.Principal{
				ObjectMeta: v1.ObjectMeta{
					Name: "github_user1",
				},
				DisplayName: "github_user1",
				Provider:    "github",
			},
			existingUser: &v3.User{
				ObjectMeta: v1.ObjectMeta{
					Name: "user1",
					UID:  "uid1",
				},
				PrincipalIDs: []string{"local://user"},
			},
			principalUser:  nil,
			principalError: nil,
			expectedUser: &v3.User{
				ObjectMeta: v1.ObjectMeta{
					Name: "user1",
					UID:  "uid1",
				},
				PrincipalIDs: []string{"local://user", "github_user1"},
			},
			expectedError:    nil,
			expectedToUpdate: true,
		},
		{
			name:   "user retrieval fails",
			userID: "user1",
			principal: v3.Principal{
				ObjectMeta: v1.ObjectMeta{
					Name: "user1",
				},
				Provider: "github",
			},
			existingUser:     nil,
			existingError:    errors.New("user not found"),
			expectedError:    errors.New("user not found"),
			expectedToUpdate: false,
		},
		{
			name:   "principal conflict with another user",
			userID: "user1",
			principal: v3.Principal{
				ObjectMeta: v1.ObjectMeta{
					Name: "github_user1",
				},
				DisplayName: "github_user1",
				Provider:    "github",
			},
			existingUser: &v3.User{
				ObjectMeta: v1.ObjectMeta{
					Name: "user1",
					UID:  "uid1",
				},
				PrincipalIDs: []string{"local://user"},
			},
			principalUser: &v3.User{
				ObjectMeta: v1.ObjectMeta{
					Name: "user2",
					UID:  "uid2",
				},
				PrincipalIDs: []string{"local://user", "github_user1"},
			},
			principalError: nil,
			expectedUser: &v3.User{
				ObjectMeta: v1.ObjectMeta{
					Name: "user1",
					UID:  "uid1",
				},
				PrincipalIDs: []string{"local://user"},
			},
			expectedError:    errors.New("refusing to set principal on user that is already bound to another user"),
			expectedToUpdate: false,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			userControllerMock := wranglerfake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)
			mockUserIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
			indexers := map[string]cache.IndexFunc{
				userByPrincipalIndex: userByPrincipal,
			}
			mockUserIndexer.AddIndexers(indexers)

			userControllerMock.EXPECT().Get(test.userID, gomock.Any()).Return(test.existingUser, test.existingError)
			if test.principalUser != nil {
				userControllerMock.EXPECT().List(gomock.Any()).Return(&v3.UserList{Items: []v3.User{*test.principalUser}}, test.principalError).AnyTimes()
			} else {
				userControllerMock.EXPECT().List(gomock.Any()).Return(&v3.UserList{}, test.principalError).AnyTimes()
			}
			userControllerMock.EXPECT().Update(gomock.Any()).AnyTimes().DoAndReturn(func(user *v3.User) (*v3.User, error) {
				u := user.DeepCopy()
				return u, nil
			})

			um := &userManager{
				users:       userControllerMock,
				userIndexer: mockUserIndexer,
			}

			result, err := um.SetPrincipalOnCurrentUserByUserID(test.userID, test.principal)

			if test.expectedError != nil {
				assert.EqualError(t, err, test.expectedError.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, test.expectedUser, result)
			}
		})
	}
}
