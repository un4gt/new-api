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
import { zodResolver } from '@hookform/resolvers/zod'
import { CheckCircle2, Loader2 } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import type { z } from 'zod'

import { Dialog } from '@/components/dialog'
import { PasswordInput } from '@/components/password-input'
import { Turnstile } from '@/components/turnstile'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  activateRegistrationInvite,
  getRegistrationInviteActivation,
  register,
  wechatLoginByCode,
} from '@/features/auth/api'
import { LegalConsent } from '@/features/auth/components/legal-consent'
import { OAuthProviders } from '@/features/auth/components/oauth-providers'
import { registerFormSchema } from '@/features/auth/constants'
import { useAuthRedirect } from '@/features/auth/hooks/use-auth-redirect'
import { useEmailVerification } from '@/features/auth/hooks/use-email-verification'
import { useTurnstile } from '@/features/auth/hooks/use-turnstile'
import {
  getAffiliateCode,
  saveAffiliateCode,
} from '@/features/auth/lib/storage'
import { useStatus } from '@/hooks/use-status'
import { cn } from '@/lib/utils'

function getInviteActionLabel(
  inviteActivated: boolean,
  inviteLoading: boolean,
  t: (key: string) => string
) {
  if (inviteActivated) return t('Activated')
  if (inviteLoading) return t('Activating...')
  return t('Activate')
}

function getInviteActionIcon(inviteActivated: boolean, inviteLoading: boolean) {
  if (inviteLoading) return <Loader2 className='h-4 w-4 animate-spin' />
  if (inviteActivated) return <CheckCircle2 className='h-4 w-4' />
  return null
}

function getVerificationButtonContent(
  isActive: boolean,
  isSendingCode: boolean,
  secondsLeft: number,
  t: (key: string, options?: Record<string, unknown>) => string
) {
  if (isActive) return t('Resend ({{seconds}}s)', { seconds: secondsLeft })
  if (isSendingCode) return <Loader2 className='h-4 w-4 animate-spin' />
  return t('Send code')
}

export function SignUpForm({
  className,
  ...props
}: React.HTMLAttributes<HTMLFormElement>) {
  const { t } = useTranslation()
  const [isLoading, setIsLoading] = useState(false)
  const [verificationCode, setVerificationCode] = useState('')
  const [agreedToLegal, setAgreedToLegal] = useState(false)
  const [wechatCode, setWeChatCode] = useState('')
  const [isWeChatDialogOpen, setIsWeChatDialogOpen] = useState(false)
  const [isWeChatSubmitting, setIsWeChatSubmitting] = useState(false)
  const [inviteCode, setInviteCode] = useState('')
  const [inviteActivated, setInviteActivated] = useState(false)
  const [inviteActivationExpiresAt, setInviteActivationExpiresAt] = useState(0)
  const [inviteLoading, setInviteLoading] = useState(false)
  const legalConsentErrorMessage = t('Please agree to the legal terms first')

  const { status } = useStatus()
  const {
    isTurnstileEnabled,
    turnstileSiteKey,
    turnstileToken,
    setTurnstileToken,
    validateTurnstile,
  } = useTurnstile()
  const { redirectToLogin, handleLoginSuccess } = useAuthRedirect()
  const {
    isSending: isSendingCode,
    secondsLeft,
    isActive,
    sendCode,
  } = useEmailVerification({
    turnstileToken,
    validateTurnstile,
  })

  const form = useForm<z.infer<typeof registerFormSchema>>({
    resolver: zodResolver(registerFormSchema),
    defaultValues: {
      username: '',
      email: '',
      password: '',
      confirmPassword: '',
    },
  })

  const emailValue = form.watch('email')
  const emailVerificationRequired = !!status?.email_verification
  const hasUserAgreement = Boolean(status?.user_agreement_enabled)
  const hasPrivacyPolicy = Boolean(status?.privacy_policy_enabled)
  const requiresLegalConsent = hasUserAgreement || hasPrivacyPolicy
  const oauthRegisterEnabled =
    status?.oauth_register_enabled ??
    status?.data?.oauth_register_enabled ??
    true
  const registrationInviteRequired = Boolean(
    status?.registration_invite_required ||
    status?.data?.registration_invite_required
  )
  const inviteGateReady = !registrationInviteRequired || inviteActivated
  const hasWeChatLogin = Boolean(status?.wechat_login)
  const turnstileReady = !isTurnstileEnabled || Boolean(turnstileToken)

  const wechatQrCodeUrl = useMemo(() => {
    return (
      status?.wechat_qrcode ||
      status?.wechat_qr_code ||
      status?.wechat_qrcode_image_url ||
      status?.wechat_qr_code_image_url ||
      status?.wechat_account_qrcode_image_url ||
      status?.WeChatAccountQRCodeImageURL ||
      status?.data?.wechat_qrcode ||
      status?.data?.WeChatAccountQRCodeImageURL ||
      ''
    )
  }, [status])

  useEffect(() => {
    if (requiresLegalConsent) {
      setAgreedToLegal(false)
    } else {
      setAgreedToLegal(true)
    }
  }, [requiresLegalConsent])

  useEffect(() => {
    const aff = new URLSearchParams(window.location.search).get('aff')?.trim()
    if (aff) {
      saveAffiliateCode(aff)
    }
  }, [])

  useEffect(() => {
    const inviteFromQuery = new URLSearchParams(window.location.search)
      .get('invite')
      ?.trim()
    if (inviteFromQuery) {
      setInviteCode(inviteFromQuery)
    }
  }, [])

  useEffect(() => {
    if (!registrationInviteRequired) {
      setInviteActivated(false)
      setInviteActivationExpiresAt(0)
      return
    }

    let cancelled = false
    async function loadActivation() {
      try {
        const res = await getRegistrationInviteActivation()
        if (cancelled) return
        if (res.success && res.data?.active) {
          setInviteActivated(true)
          setInviteActivationExpiresAt(res.data.expires_at || 0)
        } else {
          setInviteActivated(false)
          setInviteActivationExpiresAt(0)
        }
      } catch {
        if (!cancelled) {
          setInviteActivated(false)
          setInviteActivationExpiresAt(0)
        }
      }
    }

    loadActivation()
    return () => {
      cancelled = true
    }
  }, [registrationInviteRequired])

  async function onSubmit(data: z.infer<typeof registerFormSchema>) {
    if (!inviteGateReady) {
      toast.info(t('Please activate an invitation code first'))
      return
    }

    if (requiresLegalConsent && !agreedToLegal) {
      toast.error(legalConsentErrorMessage)
      return
    }

    // Validate email verification if required
    if (emailVerificationRequired) {
      if (!data.email) {
        toast.error(t('Please enter your email'))
        return
      }
      if (!verificationCode) {
        toast.error(t('Please enter the verification code'))
        return
      }
    }

    if (!validateTurnstile()) return

    setIsLoading(true)
    try {
      const res = await register({
        username: data.username,
        password: data.password,
        email: data.email || undefined,
        verification_code: verificationCode || undefined,
        aff_code: getAffiliateCode(),
        turnstile: turnstileToken,
      })

      if (res?.success) {
        toast.success(t('Account created! Please sign in'))
        redirectToLogin()
      } else {
        toast.error(res?.message || t('Failed to create account'))
      }
    } catch {
      // Errors are handled by global interceptor
    } finally {
      setIsLoading(false)
    }
  }

  async function handleSendVerificationCode() {
    await sendCode(emailValue || '')
  }

  const handleOpenWeChatDialog = () => {
    if (!inviteGateReady) {
      toast.info(t('Please activate an invitation code first'))
      return
    }

    if (requiresLegalConsent && !agreedToLegal) {
      toast.error(legalConsentErrorMessage)
      return
    }

    setIsWeChatDialogOpen(true)
  }

  const handleWeChatDialogChange = (open: boolean) => {
    setIsWeChatDialogOpen(open)
    if (!open) {
      setWeChatCode('')
      setIsWeChatSubmitting(false)
    }
  }

  async function handleWeChatLogin() {
    if (!wechatCode.trim()) {
      toast.error(t('Please enter the verification code'))
      return
    }

    setIsWeChatSubmitting(true)
    try {
      const res = await wechatLoginByCode(wechatCode)
      if (res?.success) {
        await handleLoginSuccess(res.data as { id?: number } | null)
        toast.success(t('Signed in via WeChat'))
        handleWeChatDialogChange(false)
      } else {
        toast.error(res?.message || t('Login failed'))
      }
    } catch {
      toast.error(t('Login failed'))
    } finally {
      setIsWeChatSubmitting(false)
    }
  }

  async function handleActivateInviteCode() {
    if (!registrationInviteRequired) return
    if (!inviteCode.trim()) {
      toast.info(t('Please enter invitation code'))
      return
    }

    setInviteLoading(true)
    try {
      const res = await activateRegistrationInvite(inviteCode)
      if (res.success && res.data?.active) {
        setInviteActivated(true)
        setInviteActivationExpiresAt(res.data.expires_at || 0)
        toast.success(res.message || t('Invitation code activated'))
      } else {
        setInviteActivated(false)
        setInviteActivationExpiresAt(0)
        toast.error(res.message || t('Failed to activate invitation code'))
      }
    } catch {
      toast.error(t('Failed to activate invitation code'))
    } finally {
      setInviteLoading(false)
    }
  }

  function formatInviteActivationTime(timestamp: number) {
    if (!timestamp) return ''
    return new Date(timestamp * 1000).toLocaleString()
  }

  return (
    <Form {...form}>
      <form
        onSubmit={form.handleSubmit(onSubmit)}
        className={cn('grid gap-4', className)}
        {...props}
      >
        {registrationInviteRequired && (
          <div className='border-border bg-muted/30 grid gap-3 rounded-lg border p-3'>
            <div className='grid gap-1'>
              <Label htmlFor='registration-invite-code'>
                {t('Invitation Code')}
              </Label>
              <p className='text-muted-foreground text-sm'>
                {t('Registration requires an activated invitation code.')}
              </p>
            </div>
            <div className='flex flex-col gap-2 sm:flex-row'>
              <Input
                id='registration-invite-code'
                value={inviteCode}
                placeholder={t('Please enter invitation code')}
                onChange={(event) => setInviteCode(event.target.value)}
                disabled={inviteLoading || inviteActivated}
              />
              <Button
                type='button'
                variant={inviteActivated ? 'outline' : 'default'}
                onClick={handleActivateInviteCode}
                disabled={inviteLoading || inviteActivated}
                className='sm:w-auto'
              >
                {getInviteActionIcon(inviteActivated, inviteLoading)}
                {getInviteActionLabel(inviteActivated, inviteLoading, t)}
              </Button>
            </div>
            <p className='text-muted-foreground text-sm'>
              {inviteActivated
                ? t('Invitation code activated, valid until {{time}}', {
                    time: formatInviteActivationTime(inviteActivationExpiresAt),
                  })
                : t(
                    'Activate the invitation code before registration or OAuth.'
                  )}
            </p>
          </div>
        )}

        {/* Username Field */}
        <FormField
          control={form.control}
          name='username'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Username')}</FormLabel>
              <FormControl>
                <Input placeholder={t('Enter your username')} {...field} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        {/* Password Field */}
        <FormField
          control={form.control}
          name='password'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Password')}</FormLabel>
              <FormControl>
                <PasswordInput
                  placeholder={t('Enter password (8-20 characters)')}
                  {...field}
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        {/* Confirm Password Field */}
        <FormField
          control={form.control}
          name='confirmPassword'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Confirm password')}</FormLabel>
              <FormControl>
                <PasswordInput placeholder={t('Confirm password')} {...field} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        {/* Email Verification Section */}
        {emailVerificationRequired && (
          <>
            {/* Email Field */}
            <FormField
              control={form.control}
              name='email'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>
                    {t('Email (required for verification)')}
                  </FormLabel>
                  <FormControl>
                    <Input
                      placeholder={t('name@example.com')}
                      type='email'
                      {...field}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            {/* Verification Code Field */}
            <div className='flex items-end gap-2'>
              <div className='flex-1'>
                <Input
                  placeholder={t('Verification code')}
                  value={verificationCode}
                  onChange={(e) => setVerificationCode(e.target.value)}
                />
              </div>
              <Button
                variant='outline'
                type='button'
                disabled={
                  isLoading ||
                  isSendingCode ||
                  isActive ||
                  !emailValue ||
                  !turnstileReady
                }
                onClick={handleSendVerificationCode}
              >
                {getVerificationButtonContent(
                  isActive,
                  isSendingCode,
                  secondsLeft,
                  t
                )}
              </Button>
            </div>
          </>
        )}

        {/* Turnstile */}
        {isTurnstileEnabled && (
          <div className='mt-2'>
            <Turnstile
              siteKey={turnstileSiteKey}
              onVerify={setTurnstileToken}
            />
          </div>
        )}

        <LegalConsent
          status={status}
          checked={agreedToLegal}
          onCheckedChange={setAgreedToLegal}
          className='mt-1'
        />

        {/* Submit Button */}
        <Button
          type='submit'
          className='mt-2 w-full justify-center gap-2'
          disabled={
            isLoading ||
            !inviteGateReady ||
            (requiresLegalConsent && !agreedToLegal) ||
            !turnstileReady
          }
        >
          {isLoading ? <Loader2 className='h-4 w-4 animate-spin' /> : null}
          {t('Create account')}
        </Button>

        {oauthRegisterEnabled && (
          <OAuthProviders
            status={status}
            disabled={
              isLoading ||
              !inviteGateReady ||
              (requiresLegalConsent && !agreedToLegal)
            }
            onWeChatLogin={hasWeChatLogin ? handleOpenWeChatDialog : undefined}
            isWeChatLoading={isWeChatSubmitting}
            className='pt-2'
          />
        )}
      </form>

      {hasWeChatLogin && (
        <Dialog
          open={isWeChatDialogOpen}
          onOpenChange={handleWeChatDialogChange}
          title={t('WeChat sign in')}
          description={t(
            'Scan the QR code to follow the official account and reply with “验证码” to receive your verification code.'
          )}
          contentClassName='max-w-sm'
          headerClassName='text-left'
          contentHeight='auto'
          bodyClassName='space-y-4'
          footer={
            <>
              <Button
                type='button'
                variant='outline'
                onClick={() => handleWeChatDialogChange(false)}
                disabled={isWeChatSubmitting}
              >
                {t('Cancel')}
              </Button>
              <Button
                type='button'
                onClick={handleWeChatLogin}
                disabled={
                  isWeChatSubmitting ||
                  !wechatCode.trim() ||
                  (requiresLegalConsent && !agreedToLegal)
                }
                className='gap-2'
              >
                {isWeChatSubmitting ? (
                  <Loader2 className='h-4 w-4 animate-spin' />
                ) : null}
                {t('Confirm')}
              </Button>
            </>
          }
        >
          {wechatQrCodeUrl ? (
            <div className='flex justify-center'>
              <img
                src={wechatQrCodeUrl}
                alt={t('WeChat login QR code')}
                className='h-40 w-40 rounded-md border object-contain'
              />
            </div>
          ) : (
            <p className='text-muted-foreground text-sm'>
              {t('QR code is not configured. Please contact support.')}
            </p>
          )}
          <div className='grid gap-2'>
            <Label htmlFor='wechat-code'>{t('Verification code')}</Label>
            <Input
              id='wechat-code'
              placeholder={t('Enter the verification code')}
              value={wechatCode}
              onChange={(event) => setWeChatCode(event.target.value)}
              autoComplete='one-time-code'
            />
          </div>
        </Dialog>
      )}
    </Form>
  )
}
