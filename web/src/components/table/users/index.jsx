/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React from 'react';
import { Tabs, Tag } from '@douyinfe/semi-ui';
import CardPro from '../../common/ui/CardPro';
import UsersTable from './UsersTable';
import UsersActions from './UsersActions';
import UsersFilters from './UsersFilters';
import UsersDescription from './UsersDescription';
import AddUserModal from './modals/AddUserModal';
import EditUserModal from './modals/EditUserModal';
import {
  useUsersData,
  USER_VIEW_MODES,
} from '../../../hooks/users/useUsersData';
import { useIsMobile } from '../../../hooks/common/useIsMobile';
import { createCardProPagination } from '../../../helpers/utils';

const UsersPage = () => {
  const usersData = useUsersData();
  const isMobile = useIsMobile();

  const {
    // Modal state
    showAddUser,
    showEditUser,
    editingUser,
    setShowAddUser,
    closeAddUser,
    closeEditUser,
    refresh,

    // Form state
    formInitValues,
    setFormApi,
    searchUsers,
    loadUsers,
    activePage,
    pageSize,
    groupOptions,
    loading,
    searching,

    // Description state
    compactMode,
    setCompactMode,
    viewMode,
    setViewMode,
    isBlockedView,

    // Translation
    t,
  } = usersData;

  const usersTabs = (
    <Tabs
      type='card'
      activeKey={viewMode}
      onChange={(key) => setViewMode(key)}
      className='mb-2'
    >
      <Tabs.TabPane
        itemKey={USER_VIEW_MODES.ALL}
        tab={
          <span className='flex items-center gap-2'>
            {t('用户管理')}
            <Tag
              color={viewMode === USER_VIEW_MODES.ALL ? 'red' : 'grey'}
              shape='circle'
            >
              {viewMode === USER_VIEW_MODES.ALL ? usersData.userCount : '-'}
            </Tag>
          </span>
        }
      />
      <Tabs.TabPane
        itemKey={USER_VIEW_MODES.BLOCKED}
        tab={
          <span className='flex items-center gap-2'>
            {t('屏蔽用户管理')}
            <Tag
              color={viewMode === USER_VIEW_MODES.BLOCKED ? 'red' : 'grey'}
              shape='circle'
            >
              {viewMode === USER_VIEW_MODES.BLOCKED ? usersData.userCount : '-'}
            </Tag>
          </span>
        }
      />
    </Tabs>
  );

  return (
    <>
      <AddUserModal
        refresh={refresh}
        visible={showAddUser}
        handleClose={closeAddUser}
      />

      <EditUserModal
        refresh={refresh}
        visible={showEditUser}
        handleClose={closeEditUser}
        editingUser={editingUser}
      />

      <CardPro
        type='type3'
        descriptionArea={
          <UsersDescription
            compactMode={compactMode}
            setCompactMode={setCompactMode}
            isBlockedView={isBlockedView}
            t={t}
          />
        }
        tabsArea={usersTabs}
        actionsArea={
          <div className='flex flex-col md:flex-row justify-between items-center gap-2 w-full'>
            <UsersActions setShowAddUser={setShowAddUser} t={t} />

            <UsersFilters
              formInitValues={formInitValues}
              setFormApi={setFormApi}
              searchUsers={searchUsers}
              loadUsers={loadUsers}
              activePage={activePage}
              pageSize={pageSize}
              groupOptions={groupOptions}
              loading={loading}
              searching={searching}
              t={t}
            />
          </div>
        }
        paginationArea={createCardProPagination({
          currentPage: usersData.activePage,
          pageSize: usersData.pageSize,
          total: usersData.userCount,
          onPageChange: usersData.handlePageChange,
          onPageSizeChange: usersData.handlePageSizeChange,
          isMobile: isMobile,
          t: usersData.t,
        })}
        t={usersData.t}
      >
        <UsersTable {...usersData} />
      </CardPro>
    </>
  );
};

export default UsersPage;
