// AccountMenu — menu avatar ở header desktop: một mục Đăng xuất.
// Bản scaffold chưa có mục khác; khi có thêm tính năng tài khoản thì thêm item vào đây.
import { toast } from '@noverse/ui'
import { ActionMenu, Avatar, Icon } from '@noverse/ui'
import { logout } from '../api/client'
import { useSession } from '../api/session'
import { t } from '../i18n/vi'

export function AccountMenu() {
  const { user, setUser } = useSession()
  if (!user) return null
  const name = user.name || user.username
  return (
    <ActionMenu
      alwaysVisible
      label={t.account.menuLabel}
      trigger={<Avatar name={name} size={26} />}
      items={[
        {
          id: 'logout', label: t.account.logout, danger: true, icon: <Icon name="logout" />,
          onSelect: () => {
            void logout()
              .then(() => setUser(null))
              .catch(() => toast(t.account.logoutFailed, 'err'))
          },
        },
      ]}
    />
  )
}
