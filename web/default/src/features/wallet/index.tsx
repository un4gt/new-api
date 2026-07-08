/*
Copyright (C) 2023-2026 QuantumNous

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
import { useState, useEffect, useCallback } from 'react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { getSelf } from '@/lib/api'

import { AffiliateRewardsCard } from './components/affiliate-rewards-card'
import { BillingHistoryDialog } from './components/dialogs/billing-history-dialog'
import { RedemptionCard } from './components/redemption-card'
import { TransferDialog } from './components/dialogs/transfer-dialog'
import { WalletStatsCard } from './components/wallet-stats-card'
import { useTopupInfo, useAffiliate, useRedemption } from './hooks'
import type { UserWalletData } from './types'

interface WalletProps {
  initialShowHistory?: boolean
  redemptionOnly?: boolean
}

export function Wallet(props: WalletProps) {
  const { t } = useTranslation()
  const [user, setUser] = useState<UserWalletData | null>(null)
  const [userLoading, setUserLoading] = useState(true)
  const [redemptionCode, setRedemptionCode] = useState('')
  const [transferDialogOpen, setTransferDialogOpen] = useState(false)
  const [billingDialogOpen, setBillingDialogOpen] = useState(false)

  const { topupInfo, loading: topupLoading } = useTopupInfo()
  const {
    affiliateLink,
    loading: affiliateLoading,
    transferQuota,
    transferring,
  } = useAffiliate()
  const { redeeming, redeemCode } = useRedemption()

  // Fetch and refresh user data
  const fetchUser = useCallback(async () => {
    try {
      setUserLoading(true)
      const response = await getSelf()
      if (response.success && response.data) {
        setUser(response.data as UserWalletData)
      }
    } catch (error) {
      // eslint-disable-next-line no-console
      console.error('Failed to fetch user data:', error)
    } finally {
      setUserLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchUser()
  }, [fetchUser])

  useEffect(() => {
    if (props.initialShowHistory) {
      setBillingDialogOpen(true)
      window.history.replaceState({}, '', window.location.pathname)
    }
  }, [props.initialShowHistory])

  // Handle redemption
  const handleRedeem = async () => {
    if (!redemptionCode) return

    const success = await redeemCode(redemptionCode)
    if (success) {
      setRedemptionCode('')
      await fetchUser()
    }
  }

  // Handle transfer
  const handleTransfer = async (amount: number) => {
    const success = await transferQuota(amount)
    if (success) {
      await fetchUser()
    }
    return success
  }

  const walletTitle = props.redemptionOnly ? 'Redemption Code' : 'Wallet'

  return (
    <>
      <SectionPageLayout>
        <SectionPageLayout.Title>{t(walletTitle)}</SectionPageLayout.Title>
        <SectionPageLayout.Content>
          <div className='mx-auto flex w-full max-w-7xl flex-col gap-4 sm:gap-5'>
            <WalletStatsCard user={user} loading={userLoading} />

            <RedemptionCard
              redemptionCode={redemptionCode}
              onRedemptionCodeChange={setRedemptionCode}
              onRedeem={handleRedeem}
              redeeming={redeeming}
              topupLink={topupInfo?.topup_link}
              redemptionEnabled={topupInfo?.enable_redemption !== false}
              loading={topupLoading}
              onOpenBilling={() => setBillingDialogOpen(true)}
            />

            {!props.redemptionOnly && (
              <AffiliateRewardsCard
                user={user}
                affiliateLink={affiliateLink}
                onTransfer={() => setTransferDialogOpen(true)}
                complianceConfirmed={
                  topupInfo?.payment_compliance_confirmed !== false
                }
                loading={affiliateLoading}
              />
            )}
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      {!props.redemptionOnly && (
        <TransferDialog
          open={transferDialogOpen}
          onOpenChange={setTransferDialogOpen}
          onConfirm={handleTransfer}
          availableQuota={user?.aff_quota ?? 0}
          transferring={transferring}
        />
      )}

      <BillingHistoryDialog
        open={billingDialogOpen}
        onOpenChange={setBillingDialogOpen}
      />
    </>
  )
}
