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
import { API, showError, showSuccess, timestamp2string } from '../../../../helpers';
import { useIsMobile } from '../../../../hooks/common/useIsMobile';
import {
  Avatar,
  Button,
  Card,
  Empty,
  Form,
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
  IconRefresh,
  IconSave,
  IconSafe,
} from '@douyinfe/semi-icons';

const { Text, Title } = Typography;

const RegistrationInviteModal = ({ visible, handleClose, t }) => {
  const formApiRef = useRef(null);
  const isMobile = useIsMobile();
  const [loading, setLoading] = useState(false);
  const [creating, setCreating] = useState(false);
  const [revokingId, setRevokingId] = useState(null);
  const [invites, setInvites] = useState([]);
  const [createdInviteCode, setCreatedInviteCode] = useState('');

  const getInitValues = () => ({
    note: '',
    expires_at: null,
  });

  const loadInvites = async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/registration-invite/?p=1&page_size=20');
      const { success, message, data } = res.data;
      if (success) {
        setInvites(data.items || []);
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
    setCreatedInviteCode('');
    loadInvites();
    formApiRef.current?.setValues(getInitValues());
  }, [visible]);

  const submit = async (values) => {
    setCreating(true);
    try {
      const payload = {
        note: values.note || '',
        expires_at: values.expires_at
          ? Math.floor(values.expires_at.getTime() / 1000)
          : 0,
      };
      const res = await API.post('/api/registration-invite/', payload);
      const { success, message, data } = res.data;
      if (success) {
        setCreatedInviteCode(data.invite_code || '');
        formApiRef.current?.setValues(getInitValues());
        showSuccess(t('邀请码创建成功'));
        loadInvites();
      } else {
        showError(message);
      }
    } catch (error) {
      showError(t('邀请码创建失败，请重试'));
    } finally {
      setCreating(false);
    }
  };

  const copyInviteCode = async (code) => {
    if (!code) {
      return;
    }
    try {
      await navigator.clipboard.writeText(code);
      showSuccess(t('邀请码已复制到剪贴板'));
    } catch (error) {
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
        loadInvites();
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
    if (record.expires_at !== 0 && record.expires_at < Math.floor(Date.now() / 1000)) {
      return <Tag color='orange'>{t('已过期')}</Tag>;
    }
    return <Tag color='green'>{t('可用')}</Tag>;
  };

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
        title: t('备注'),
        dataIndex: 'note',
        render: (text) => text || t('无'),
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
    [revokingId, t],
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
              onClick={loadInvites}
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
          <div className='p-2 space-y-3'>
            <Card className='!rounded-2xl shadow-sm border-0'>
              <div className='flex items-center mb-2'>
                <Avatar size='small' color='orange' className='mr-2 shadow-md'>
                  <IconSafe size={16} />
                </Avatar>
                <div>
                  <Text className='text-lg font-medium'>{t('创建新邀请码')}</Text>
                  <div className='text-xs text-gray-600'>
                    {t('邀请码只展示一次，请立即复制保存')}
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
                <Col span={24}>
                  <Form.DatePicker
                    field='expires_at'
                    label={t('过期时间')}
                    type='dateTime'
                    placeholder={t('选择过期时间（可选，留空为永不过期）')}
                    showClear
                    style={{ width: '100%' }}
                  />
                </Col>
              </Row>

              {createdInviteCode && (
                <div className='mt-4 rounded-2xl border border-green-200 bg-green-50 p-4'>
                  <div className='flex flex-col gap-3 md:flex-row md:items-center md:justify-between'>
                    <div>
                      <div className='text-sm text-green-800'>{t('新邀请码')}</div>
                      <div className='mt-1 font-mono text-base text-green-900 break-all'>
                        {createdInviteCode}
                      </div>
                    </div>
                    <Button
                      theme='solid'
                      type='primary'
                      icon={<IconCopy />}
                      onClick={() => copyInviteCode(createdInviteCode)}
                    >
                      {t('复制邀请码')}
                    </Button>
                  </div>
                </div>
              )}
            </Card>

            <Card className='!rounded-2xl shadow-sm border-0'>
              <div className='flex items-center justify-between mb-3'>
                <div>
                  <Text className='text-lg font-medium'>{t('最近邀请码')}</Text>
                  <div className='text-xs text-gray-600'>
                    {t('只展示最近 20 条邀请码记录')}
                  </div>
                </div>
              </div>

              {invites.length === 0 ? (
                <Empty description={t('暂无邀请码')} />
              ) : (
                <Table
                  dataSource={invites}
                  columns={columns}
                  pagination={false}
                  size='small'
                  rowKey='id'
                />
              )}
            </Card>
          </div>
        </Form>
      </Spin>
    </SideSheet>
  );
};

export default RegistrationInviteModal;
