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

import React, { useEffect, useMemo, useRef, useState } from 'react';
import {
  API,
  copy,
  downloadTextAsFile,
  showError,
  showSuccess,
  timestamp2string,
} from '../../../../helpers';
import { useIsMobile } from '../../../../hooks/common/useIsMobile';
import {
  Avatar,
  Button,
  Card,
  Empty,
  Form,
  Input,
  Row,
  Col,
  SideSheet,
  Space,
  Spin,
  Table,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import {
  IconClose,
  IconCopy,
  IconDownload,
  IconRefresh,
  IconSave,
  IconSafe,
  IconSearch,
} from '@douyinfe/semi-icons';

const { Text, Title } = Typography;
const DEFAULT_PAGE_SIZE = 20;

const RegistrationInviteModal = ({ visible, handleClose, t }) => {
  const formApiRef = useRef(null);
  const isMobile = useIsMobile();
  const [loading, setLoading] = useState(false);
  const [creating, setCreating] = useState(false);
  const [revokingId, setRevokingId] = useState(null);
  const [invites, setInvites] = useState([]);
  const [createdInviteCodes, setCreatedInviteCodes] = useState([]);
  const [searchKeyword, setSearchKeyword] = useState('');
  const [activePage, setActivePage] = useState(1);
  const [pageSize, setPageSize] = useState(DEFAULT_PAGE_SIZE);
  const [inviteCount, setInviteCount] = useState(0);

  const getInitValues = () => ({
    note: '',
    expires_at: null,
    count: 1,
  });

  const loadInvites = async (
    page = 1,
    size = DEFAULT_PAGE_SIZE,
    keywordOverride,
  ) => {
    setLoading(true);
    try {
      const keyword = (keywordOverride ?? searchKeyword).trim();
      const params = new URLSearchParams({
        p: String(page),
        page_size: String(size),
      });
      if (keyword) {
        params.set('keyword', keyword);
      }
      const res = await API.get(`/api/registration-invite/?${params}`);
      const { success, message, data } = res.data;
      if (success) {
        setInvites(data.items || []);
        setActivePage(data.page <= 0 ? 1 : data.page);
        setInviteCount(data.total || 0);
      } else {
        showError(message);
      }
    } catch (error) {
      showError(t('邀请码列表加载失败，请重试'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (!visible) {
      return;
    }
    setCreatedInviteCodes([]);
    setActivePage(1);
    setPageSize(DEFAULT_PAGE_SIZE);
    setInviteCount(0);
    setSearchKeyword('');
    loadInvites(1, DEFAULT_PAGE_SIZE, '');
    formApiRef.current?.setValues(getInitValues());
  }, [visible]);

  const submit = async (values) => {
    setCreating(true);
    try {
      const count = parseInt(values.count, 10) || 0;
      const payload = {
        note: values.note || '',
        expires_at: values.expires_at
          ? Math.floor(values.expires_at.getTime() / 1000)
          : 0,
        count,
      };
      const res = await API.post('/api/registration-invite/', payload);
      const { success, message, data } = res.data;
      if (success) {
        const inviteCodes =
          data.invite_codes || (data.invite_code ? [data.invite_code] : []);
        setCreatedInviteCodes(inviteCodes);
        formApiRef.current?.setValues(getInitValues());
        showSuccess(t('邀请码创建成功'));
        loadInvites(1, pageSize);
      } else {
        showError(message);
      }
    } catch (error) {
      showError(t('邀请码创建失败，请重试'));
    } finally {
      setCreating(false);
    }
  };

  const searchInvites = () => {
    setActivePage(1);
    loadInvites(1, pageSize, searchKeyword);
  };

  const resetSearch = () => {
    setSearchKeyword('');
    setActivePage(1);
    loadInvites(1, pageSize, '');
  };

  const handlePageChange = (page) => {
    setActivePage(page);
    loadInvites(page, pageSize);
  };

  const handlePageSizeChange = (size) => {
    setPageSize(size);
    setActivePage(1);
    loadInvites(1, size);
  };

  const copyInviteCodes = async (codes) => {
    const text = Array.isArray(codes)
      ? codes.filter(Boolean).join('\n')
      : codes;
    if (!text) {
      return;
    }
    if (await copy(text)) {
      showSuccess(t('邀请码已复制到剪贴板'));
    } else {
      showError(t('复制失败，请手动复制'));
    }
  };

  const revokeInvite = async (inviteId) => {
    setRevokingId(inviteId);
    try {
      const res = await API.post(`/api/registration-invite/${inviteId}/revoke`);
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('邀请码已撤销'));
        loadInvites(activePage, pageSize);
      } else {
        showError(message);
      }
    } catch (error) {
      showError(t('邀请码撤销失败，请重试'));
    } finally {
      setRevokingId(null);
    }
  };

  const renderStatus = (record) => {
    if (record.status === 'used') {
      return <Tag color='red'>{t('已使用')}</Tag>;
    }
    if (record.status === 'revoked') {
      return <Tag color='grey'>{t('已撤销')}</Tag>;
    }
    if (
      record.expires_at !== 0 &&
      record.expires_at < Math.floor(Date.now() / 1000)
    ) {
      return <Tag color='orange'>{t('已过期')}</Tag>;
    }
    return <Tag color='green'>{t('可用')}</Tag>;
  };

  const createdInviteText = useMemo(
    () => createdInviteCodes.filter(Boolean).join('\n'),
    [createdInviteCodes],
  );

  const columns = useMemo(
    () => [
      {
        title: t('ID'),
        dataIndex: 'id',
        width: 72,
      },
      {
        title: t('状态'),
        dataIndex: 'status',
        width: 96,
        render: (text, record) => renderStatus(record),
      },
      {
        title: t('邀请码'),
        dataIndex: 'code',
        width: 220,
        render: (text) =>
          text ? (
            <Text copyable={{ content: text }} className='font-mono'>
              {text}
            </Text>
          ) : (
            <Text type='tertiary'>{t('无')}</Text>
          ),
      },
      {
        title: t('备注'),
        dataIndex: 'note',
        render: (text) => text || t('无'),
      },
      {
        title: t('使用次数'),
        dataIndex: 'use_count',
        width: 120,
        render: (text, record) => `${text || 0}/${record.max_uses || 1}`,
      },
      {
        title: t('使用人ID'),
        dataIndex: 'used_by',
        width: 110,
        render: (text) => (text ? text : t('无')),
      },
      {
        title: t('使用方式'),
        dataIndex: 'used_provider',
        width: 110,
        render: (text) => text || t('无'),
      },
      {
        title: t('最后使用时间'),
        dataIndex: 'used_at',
        width: 180,
        render: (text) => (text ? timestamp2string(text) : t('无')),
      },
      {
        title: t('创建时间'),
        dataIndex: 'created_at',
        width: 180,
        render: (text) => timestamp2string(text),
      },
      {
        title: t('过期时间'),
        dataIndex: 'expires_at',
        width: 180,
        render: (text) => (text === 0 ? t('永不过期') : timestamp2string(text)),
      },
      {
        title: t('操作'),
        dataIndex: 'operate',
        width: 120,
        render: (text, record) => {
          const disabled =
            record.status !== 'active' ||
            (record.expires_at !== 0 &&
              record.expires_at < Math.floor(Date.now() / 1000));
          return (
            <Button
              type='danger'
              theme='borderless'
              size='small'
              disabled={disabled}
              loading={revokingId === record.id}
              onClick={() => revokeInvite(record.id)}
            >
              {t('撤销')}
            </Button>
          );
        },
      },
    ],
    [activePage, pageSize, revokingId, searchKeyword, t],
  );

  return (
    <SideSheet
      placement='left'
      title={
        <Space>
          <Tag color='red' shape='circle'>
            {t('邀请码管理')}
          </Tag>
          <Title heading={4} className='m-0'>
            {t('邀请码管理')}
          </Title>
        </Space>
      }
      bodyStyle={{ padding: 0 }}
      visible={visible}
      width={isMobile ? '100%' : 840}
      closeIcon={null}
      onCancel={handleClose}
      footer={
        <div className='flex justify-end bg-white'>
          <Space>
            <Button
              theme='solid'
              onClick={() => formApiRef.current?.submitForm()}
              icon={<IconSave />}
              loading={creating}
            >
              {t('创建邀请码')}
            </Button>
            <Button
              theme='light'
              type='primary'
              onClick={() => loadInvites(activePage, pageSize)}
              icon={<IconRefresh />}
              loading={loading}
            >
              {t('刷新')}
            </Button>
            <Button
              theme='light'
              type='primary'
              onClick={handleClose}
              icon={<IconClose />}
            >
              {t('取消')}
            </Button>
          </Space>
        </div>
      }
    >
      <Spin spinning={loading || creating}>
        <Form
          initValues={getInitValues()}
          getFormApi={(api) => (formApiRef.current = api)}
          onSubmit={submit}
          onSubmitFail={(errs) => {
            const first = Object.values(errs)[0];
            if (first) {
              showError(Array.isArray(first) ? first[0] : first);
            }
            formApiRef.current?.scrollToError();
          }}
        >
          <div className='p-2 pb-0'>
            <Card className='!rounded-2xl shadow-sm border-0'>
              <div className='flex items-center mb-2'>
                <Avatar size='small' color='orange' className='mr-2 shadow-md'>
                  <IconSafe size={16} />
                </Avatar>
                <div>
                  <Text className='text-lg font-medium'>
                    {t('创建新邀请码')}
                  </Text>
                  <div className='text-xs text-gray-600'>
                    {t('创建后可在列表查看邀请码和使用状态')}
                  </div>
                </div>
              </div>

              <Row gutter={12}>
                <Col span={24}>
                  <Form.Input
                    field='note'
                    label={t('备注')}
                    placeholder={t('请输入备注（可选）')}
                    showClear
                  />
                </Col>
                <Col span={isMobile ? 24 : 12}>
                  <Form.DatePicker
                    field='expires_at'
                    label={t('过期时间')}
                    type='dateTime'
                    placeholder={t('选择过期时间（可选，留空为永不过期）')}
                    showClear
                    style={{ width: '100%' }}
                  />
                </Col>
                <Col span={isMobile ? 24 : 12}>
                  <Form.InputNumber
                    field='count'
                    label={t('生成数量')}
                    min={1}
                    max={100}
                    rules={[
                      { required: true, message: t('请输入生成数量') },
                      {
                        validator: (rule, v) => {
                          const num = parseInt(v, 10);
                          return num > 0 && num <= 100
                            ? Promise.resolve()
                            : Promise.reject(t('一次最多生成100个邀请码'));
                        },
                      },
                    ]}
                    style={{ width: '100%' }}
                  />
                </Col>
              </Row>

              {createdInviteText && (
                <div className='mt-4 rounded-2xl border border-green-200 bg-green-50 p-4'>
                  <div className='flex flex-col gap-3 md:flex-row md:items-center md:justify-between'>
                    <div>
                      <div className='text-sm text-green-800'>
                        {createdInviteCodes.length > 1
                          ? t('新邀请码列表')
                          : t('新邀请码')}
                      </div>
                      <div className='mt-1 max-h-40 overflow-auto whitespace-pre-wrap font-mono text-base text-green-900 break-all'>
                        {createdInviteText}
                      </div>
                    </div>
                    <Space>
                      <Button
                        theme='solid'
                        type='primary'
                        icon={<IconCopy />}
                        onClick={() => copyInviteCodes(createdInviteCodes)}
                      >
                        {createdInviteCodes.length > 1
                          ? t('复制全部')
                          : t('复制邀请码')}
                      </Button>
                      {createdInviteCodes.length > 1 && (
                        <Button
                          theme='light'
                          type='primary'
                          icon={<IconDownload />}
                          onClick={() =>
                            downloadTextAsFile(
                              createdInviteText,
                              'registration-invites.txt',
                            )
                          }
                        >
                          {t('下载')}
                        </Button>
                      )}
                    </Space>
                  </div>
                </div>
              )}
            </Card>
          </div>
        </Form>
        <div className='p-2'>
          <Card className='!rounded-2xl shadow-sm border-0'>
            <div className='flex flex-col gap-3 md:flex-row md:items-center md:justify-between mb-3'>
              <div>
                <Text className='text-lg font-medium'>{t('邀请码列表')}</Text>
                <div className='text-xs text-gray-600'>
                  {t('可按 ID、备注或邀请码值搜索')}
                </div>
              </div>
              <div className='flex flex-col md:flex-row gap-2 w-full md:w-auto'>
                <Input
                  value={searchKeyword}
                  onChange={(value) => setSearchKeyword(value)}
                  onEnterPress={searchInvites}
                  prefix={<IconSearch />}
                  placeholder={t('关键字(id、备注或邀请码)')}
                  showClear
                  size='small'
                  className='w-full md:w-64'
                />
                <Space>
                  <Button
                    type='tertiary'
                    onClick={searchInvites}
                    loading={loading}
                    size='small'
                  >
                    {t('查询')}
                  </Button>
                  <Button type='tertiary' onClick={resetSearch} size='small'>
                    {t('重置')}
                  </Button>
                </Space>
              </div>
            </div>

            {invites.length === 0 ? (
              <Empty description={t('暂无邀请码')} />
            ) : (
              <Table
                dataSource={invites}
                columns={columns}
                pagination={{
                  currentPage: activePage,
                  pageSize: pageSize,
                  total: inviteCount,
                  showSizeChanger: true,
                  pageSizeOptions: [10, 20, 50, 100],
                  onChange: (page) => handlePageChange(page),
                  onPageChange: handlePageChange,
                  onShowSizeChange: (current, size) =>
                    handlePageSizeChange(size),
                  onPageSizeChange: handlePageSizeChange,
                }}
                size='small'
                rowKey='id'
                scroll={{ x: 'max-content' }}
              />
            )}
          </Card>
        </div>
      </Spin>
    </SideSheet>
  );
};

export default RegistrationInviteModal;
