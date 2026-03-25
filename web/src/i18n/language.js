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

export const DEFAULT_LANGUAGE = 'zh-CN';

export const supportedLanguages = [DEFAULT_LANGUAGE];

export const normalizeLanguage = (language) => {
  if (!language || typeof language !== 'string') {
    return DEFAULT_LANGUAGE;
  }

  const normalized = language.trim().replace(/_/g, '-').toLowerCase();

  if (normalized === 'zh' || normalized.startsWith('zh-')) {
    return DEFAULT_LANGUAGE;
  }

  return DEFAULT_LANGUAGE;
};
