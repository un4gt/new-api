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
import { Copy, Loader2, Search, Ticket, XCircle } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { DateTimePicker } from '@/components/datetime-picker'
import { Dialog } from '@/components/dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Textarea } from '@/components/ui/textarea'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { cn } from '@/lib/utils'

import {
  createRegistrationInvite,
  getRegistrationInvites,
  revokeRegistrationInvite,
} from '../api'
import type { RegistrationInvite } from '../types'
import { useUsers } from './users-provider'

const PAGE_SIZE = 20

function formatTime(timestamp?: number) {
  if (!timestamp) return '-'
  return new Date(timestamp * 1000).toLocaleString()
}

function getStatusVariant(status: string) {
  if (status === 'active') return 'default'
  if (status === 'used') return 'secondary'
  if (status === 'revoked') return 'destructive'
  return 'outline'
}

function canRevoke(invite: RegistrationInvite) {
  return (
    invite.status === 'active' &&
    (invite.max_uses <= 0 || invite.use_count < invite.max_uses)
  )
}

function renderInviteRows(
  loading: boolean,
  invites: RegistrationInvite[],
  revokingId: number | null,
  t: (key: string) => string,
  handleRevoke: (invite: RegistrationInvite) => void
) {
  if (loading) {
    return (
      <TableRow>
        <TableCell colSpan={7} className='h-28 text-center'>
          <Loader2 className='mx-auto h-5 w-5 animate-spin' />
        </TableCell>
      </TableRow>
    )
  }

  if (invites.length === 0) {
    return (
      <TableRow>
        <TableCell
          colSpan={7}
          className='text-muted-foreground h-28 text-center'
        >
          {t('No invitation codes')}
        </TableCell>
      </TableRow>
    )
  }

  return invites.map((invite) => (
    <TableRow key={invite.id}>
      <TableCell>#{invite.id}</TableCell>
      <TableCell>
        <code className='bg-muted rounded px-1.5 py-0.5 text-xs'>
          {invite.code || '-'}
        </code>
      </TableCell>
      <TableCell>
        <Badge
          variant={getStatusVariant(invite.status)}
          className={cn(
            invite.status === 'active' && 'bg-emerald-500 text-white'
          )}
        >
          {t(invite.status)}
        </Badge>
      </TableCell>
      <TableCell>
        {invite.use_count}/{invite.max_uses || t('Unlimited')}
      </TableCell>
      <TableCell>{formatTime(invite.expires_at)}</TableCell>
      <TableCell className='max-w-48 truncate'>{invite.note || '-'}</TableCell>
      <TableCell className='text-right'>
        <Button
          type='button'
          variant='ghost'
          size='sm'
          disabled={!canRevoke(invite) || revokingId === invite.id}
          onClick={() => handleRevoke(invite)}
        >
          {revokingId === invite.id ? (
            <Loader2 className='h-4 w-4 animate-spin' />
          ) : (
            <XCircle className='h-4 w-4' />
          )}
          {t('Revoke')}
        </Button>
      </TableCell>
    </TableRow>
  ))
}

export function RegistrationInvitesDialog() {
  const { t } = useTranslation()
  const { open, setOpen } = useUsers()
  const { copyToClipboard } = useCopyToClipboard()
  const dialogOpen = open === 'registration-invites'
  const [loading, setLoading] = useState(false)
  const [creating, setCreating] = useState(false)
  const [revokingId, setRevokingId] = useState<number | null>(null)
  const [invites, setInvites] = useState<RegistrationInvite[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [keyword, setKeyword] = useState('')
  const [appliedKeyword, setAppliedKeyword] = useState('')
  const [note, setNote] = useState('')
  const [expiresAt, setExpiresAt] = useState<Date | undefined>()
  const [maxUses, setMaxUses] = useState(1)
  const [count, setCount] = useState(1)
  const [createdCodes, setCreatedCodes] = useState<string[]>([])

  const totalPages = useMemo(
    () => Math.max(1, Math.ceil(total / PAGE_SIZE)),
    [total]
  )

  const loadInvites = async () => {
    setLoading(true)
    try {
      const res = await getRegistrationInvites({
        p: page,
        page_size: PAGE_SIZE,
        keyword: appliedKeyword,
      })
      if (res.success) {
        setInvites(res.data?.items ?? [])
        setTotal(res.data?.total ?? 0)
      } else {
        toast.error(res.message || t('Failed to load invitation codes'))
      }
    } catch {
      toast.error(t('Failed to load invitation codes'))
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    if (!dialogOpen) return
    loadInvites()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [dialogOpen, page, appliedKeyword])

  const handleSearch = () => {
    setPage(1)
    setAppliedKeyword(keyword.trim())
  }

  const handleCreate = async () => {
    const safeCount = Number(count)
    if (!Number.isFinite(safeCount) || safeCount < 1 || safeCount > 100) {
      toast.error(t('Create between 1 and 100 invitation codes at a time'))
      return
    }

    const safeMaxUses = Number(maxUses)
    if (!Number.isFinite(safeMaxUses) || safeMaxUses < 1) {
      toast.error(t('Maximum uses must be at least 1'))
      return
    }

    setCreating(true)
    try {
      const res = await createRegistrationInvite({
        note: note.trim(),
        expires_at: expiresAt ? Math.floor(expiresAt.getTime() / 1000) : 0,
        max_uses: safeMaxUses,
        count: safeCount,
      })
      if (res.success) {
        const codes =
          res.data?.invite_codes ??
          (res.data?.invite_code ? [res.data.invite_code] : [])
        setCreatedCodes(codes)
        toast.success(t('Invitation code created'))
        setPage(1)
        await loadInvites()
      } else {
        toast.error(res.message || t('Failed to create invitation code'))
      }
    } catch {
      toast.error(t('Failed to create invitation code'))
    } finally {
      setCreating(false)
    }
  }

  const handleRevoke = async (invite: RegistrationInvite) => {
    setRevokingId(invite.id)
    try {
      const res = await revokeRegistrationInvite(invite.id)
      if (res.success) {
        toast.success(t('Invitation code revoked'))
        await loadInvites()
      } else {
        toast.error(res.message || t('Failed to revoke invitation code'))
      }
    } catch {
      toast.error(t('Failed to revoke invitation code'))
    } finally {
      setRevokingId(null)
    }
  }

  const copyCreatedCodes = () => {
    if (createdCodes.length === 0) return
    copyToClipboard(createdCodes.join('\n'))
  }

  return (
    <Dialog
      open={dialogOpen}
      onOpenChange={(isOpen) => !isOpen && setOpen(null)}
      title={t('Invitation Code Management')}
      description={t(
        'Create, search, and revoke registration invitation codes.'
      )}
      contentClassName='sm:max-w-5xl'
      contentHeight='min(74vh, 760px)'
      bodyClassName='grid gap-4'
      footer={
        <Button type='button' variant='outline' onClick={() => setOpen(null)}>
          {t('Close')}
        </Button>
      }
    >
      <div className='grid gap-4 lg:grid-cols-[360px_1fr]'>
        <div className='border-border/70 grid content-start gap-4 rounded-lg border p-4'>
          <div className='flex items-center gap-2'>
            <Ticket className='text-muted-foreground h-4 w-4' />
            <h3 className='text-sm font-semibold'>{t('Create Invitation')}</h3>
          </div>

          <div className='grid gap-2'>
            <Label htmlFor='invite-note'>{t('Note')}</Label>
            <Textarea
              id='invite-note'
              value={note}
              onChange={(event) => setNote(event.target.value)}
              maxLength={255}
              placeholder={t('Optional note')}
              className='min-h-20'
            />
          </div>

          <div className='grid gap-2'>
            <Label>{t('Expires At')}</Label>
            <DateTimePicker
              value={expiresAt}
              onChange={setExpiresAt}
              placeholder={t('Never expires')}
            />
          </div>

          <div className='grid grid-cols-2 gap-3'>
            <div className='grid gap-2'>
              <Label htmlFor='invite-max-uses'>{t('Max Uses')}</Label>
              <Input
                id='invite-max-uses'
                type='number'
                min={1}
                value={maxUses}
                onChange={(event) => setMaxUses(Number(event.target.value))}
              />
            </div>
            <div className='grid gap-2'>
              <Label htmlFor='invite-count'>{t('Count')}</Label>
              <Input
                id='invite-count'
                type='number'
                min={1}
                max={100}
                value={count}
                onChange={(event) => setCount(Number(event.target.value))}
              />
            </div>
          </div>

          <Button type='button' onClick={handleCreate} disabled={creating}>
            {creating ? <Loader2 className='h-4 w-4 animate-spin' /> : null}
            {t('Create')}
          </Button>

          {createdCodes.length > 0 ? (
            <div className='bg-muted/40 grid gap-3 rounded-lg border p-3'>
              <div className='flex items-center justify-between gap-2'>
                <div className='text-sm font-medium'>
                  {createdCodes.length > 1
                    ? t('New Invitation Codes')
                    : t('New Invitation Code')}
                </div>
                <Button
                  type='button'
                  variant='outline'
                  size='sm'
                  onClick={copyCreatedCodes}
                >
                  <Copy className='h-4 w-4' />
                  {t('Copy')}
                </Button>
              </div>
              <pre className='bg-background max-h-36 overflow-auto rounded-md p-2 text-xs whitespace-pre-wrap'>
                {createdCodes.join('\n')}
              </pre>
            </div>
          ) : null}
        </div>

        <div className='grid min-w-0 content-start gap-3'>
          <div className='flex flex-col gap-2 sm:flex-row'>
            <Input
              value={keyword}
              onChange={(event) => setKeyword(event.target.value)}
              placeholder={t('Search by ID, note, code, or user')}
              onKeyDown={(event) => {
                if (event.key === 'Enter') handleSearch()
              }}
            />
            <Button
              type='button'
              variant='outline'
              onClick={handleSearch}
              disabled={loading}
            >
              <Search className='h-4 w-4' />
              {t('Search')}
            </Button>
          </div>

          <div className='rounded-lg border'>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('ID')}</TableHead>
                  <TableHead>{t('Invitation Code')}</TableHead>
                  <TableHead>{t('Status')}</TableHead>
                  <TableHead>{t('Uses')}</TableHead>
                  <TableHead>{t('Expires At')}</TableHead>
                  <TableHead>{t('Note')}</TableHead>
                  <TableHead className='text-right'>{t('Actions')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {renderInviteRows(
                  loading,
                  invites,
                  revokingId,
                  t,
                  handleRevoke
                )}
              </TableBody>
            </Table>
          </div>

          <div className='flex items-center justify-between gap-2 text-sm'>
            <span className='text-muted-foreground'>
              {t('Total')}: {total}
            </span>
            <div className='flex items-center gap-2'>
              <Button
                type='button'
                variant='outline'
                size='sm'
                disabled={page <= 1 || loading}
                onClick={() => setPage((value) => Math.max(1, value - 1))}
              >
                {t('Previous')}
              </Button>
              <span className='text-muted-foreground min-w-20 text-center'>
                {page} / {totalPages}
              </span>
              <Button
                type='button'
                variant='outline'
                size='sm'
                disabled={page >= totalPages || loading}
                onClick={() =>
                  setPage((value) => Math.min(totalPages, value + 1))
                }
              >
                {t('Next')}
              </Button>
            </div>
          </div>
        </div>
      </div>
    </Dialog>
  )
}
