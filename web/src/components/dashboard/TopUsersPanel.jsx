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
import { Card, Empty, Select, Table, Typography } from '@douyinfe/semi-ui';
import { Trophy } from 'lucide-react';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import { renderQuota } from '../../helpers';

const { Text } = Typography;

const LIMIT_OPTIONS = [10, 20, 50];

const TopUsersPanel = ({
  topUsers = [],
  loading = false,
  topUsersLimit = 10,
  onLimitChange,
  topUsersSortBy = 'consume_quota',
  onSortByChange,
  CARD_PROPS,
  t,
}) => {
  const columns = [
    {
      title: t('ID'),
      dataIndex: 'id',
      width: 90,
    },
    {
      title: t('用户名'),
      dataIndex: 'username',
      width: 180,
    },
    {
      title: t('今日已消耗/今日请求次数'),
      dataIndex: 'today',
      render: (_, record) => (
        <Text>
          {renderQuota(record.today_consume_quota)} /{' '}
          {record.today_request_count?.toLocaleString?.() ||
            record.today_request_count ||
            0}
        </Text>
      ),
    },
  ];

  const dataSource = topUsers.map((item) => ({
    ...item,
    key: item.id,
  }));

  return (
    <div className='mb-4'>
      <Card
        {...CARD_PROPS}
        className='!rounded-2xl'
        title={
          <div className='flex flex-col lg:flex-row lg:items-center lg:justify-between gap-2 w-full'>
            <div className='flex items-center gap-2'>
              <Trophy size={16} />
              {t('用户调用量（次数）排行榜')}
            </div>
            <div className='flex items-center gap-2'>
              <Select
                size='small'
                value={topUsersSortBy}
                onChange={onSortByChange}
                style={{ width: 150 }}
                optionList={[
                  {
                    label: t('按今日已消耗排序'),
                    value: 'consume_quota',
                  },
                  {
                    label: t('按今日请求次数排序'),
                    value: 'request_count',
                  },
                ]}
              />
              <Select
                size='small'
                value={topUsersLimit}
                onChange={onLimitChange}
                style={{ width: 100 }}
                optionList={LIMIT_OPTIONS.map((limit) => ({
                  label: `${t('前')}${limit}`,
                  value: limit,
                }))}
              />
            </div>
          </div>
        }
      >
        <Table
          columns={columns}
          dataSource={dataSource}
          loading={loading}
          pagination={false}
          size='small'
          empty={
            <Empty
              image={<IllustrationNoResult style={{ width: 96, height: 96 }} />}
              darkModeImage={
                <IllustrationNoResultDark style={{ width: 96, height: 96 }} />
              }
              title={t('暂无用户调用数据')}
            />
          }
        />
      </Card>
    </div>
  );
};

export default TopUsersPanel;
