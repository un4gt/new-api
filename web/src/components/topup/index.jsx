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

import React, { useContext, useEffect, useState } from 'react';
import {
  Card,
  Typography,
  Form,
  Input,
  Button,
  Modal,
} from '@douyinfe/semi-ui';
import { IconGift } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import {
  API,
  renderQuota,
  showError,
  showInfo,
  showSuccess,
} from '../../helpers';
import { UserContext } from '../../context/User';

const { Text } = Typography;

const TopUp = () => {
  const { t } = useTranslation();
  const [userState, userDispatch] = useContext(UserContext);
  const [redemptionCode, setRedemptionCode] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const refreshUserQuota = async () => {
    const res = await API.get('/api/user/self');
    const { success, message, data } = res.data;
    if (success) {
      userDispatch({ type: 'login', payload: data });
    } else {
      showError(message);
    }
  };

  useEffect(() => {
    refreshUserQuota().catch(() => {});
  }, []);

  const redeem = async () => {
    if (redemptionCode.trim() === '') {
      showInfo(t('请输入兑换码！'));
      return;
    }
    setSubmitting(true);
    try {
      const res = await API.post('/api/user/topup', {
        key: redemptionCode.trim(),
      });
      const { success, message, data } = res.data;
      if (!success) {
        showError(message);
        return;
      }
      showSuccess(t('兑换成功！'));
      Modal.success({
        title: t('兑换成功！'),
        content: t('成功兑换额度：') + renderQuota(data),
        centered: true,
      });
      if (userState.user) {
        userDispatch({
          type: 'login',
          payload: {
            ...userState.user,
            quota: userState.user.quota + data,
          },
        });
      }
      setRedemptionCode('');
    } catch (error) {
      showError(t('请求失败'));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className='w-full max-w-3xl mx-auto mt-[60px] px-2'>
      <Card className='!rounded-2xl shadow-sm border-0'>
        <div className='mb-6'>
          <Text className='text-lg font-semibold'>{t('兑换码')}</Text>
          <div className='text-sm text-semi-color-text-2 mt-1'>
            {t('请输入兑换码以兑换账户额度')}
          </div>
        </div>

        <Card
          className='!rounded-xl'
          title={
            <Text type='tertiary' strong>
              {t('兑换码充值')}
            </Text>
          }
        >
          <Form initValues={{ redemptionCode }}>
            <Form.Input
              field='redemptionCode'
              noLabel
              prefix={<IconGift />}
              placeholder={t('请输入兑换码')}
              value={redemptionCode}
              onChange={setRedemptionCode}
              suffix={
                <Button
                  type='primary'
                  theme='solid'
                  onClick={redeem}
                  loading={submitting}
                >
                  {t('兑换额度')}
                </Button>
              }
              showClear
            />
          </Form>
        </Card>
      </Card>
    </div>
  );
};

export default TopUp;
