// Copyright © 2024 Kaleido, Inc.
//
// SPDX-License-Identifier: Apache-2.0
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import HourglassTopIcon from '@mui/icons-material/HourglassTop';
import { Button } from '@mui/material';
import daysjs from 'dayjs';
import relativeTime from 'dayjs/plugin/relativeTime';
import { useEffect, useState } from 'react';
import { constants } from './config';
import { TimestampDialog } from '../dialogs/Timestamp';

daysjs.extend(relativeTime);

type Props = {
  timestamp: any
};

export const EllapsedTime: React.FC<Props> = ({ timestamp }) => {

  const [displayValue, setDisplayValue] = useState<string>(daysjs(timestamp).fromNow());
  const [timestampDialogOpen, setTimestampDialogOpen] = useState(false);

  useEffect(() => {
    const intervalId = setInterval(() => {
      setDisplayValue(daysjs(timestamp).fromNow())
    }, constants.ELLAPSED_TIME_AUTO_REFRESH_FREQUENCY_SECONDS * 1000);
    return () => clearInterval(intervalId);
  }, []);

  return (
    <>
      <Button
        size="small"
        startIcon={<HourglassTopIcon />}
        onClick={() => setTimestampDialogOpen(true)}>
        {displayValue}
      </Button>
      <TimestampDialog
        date={new Date(timestamp)}
        dialogOpen={timestampDialogOpen}
        setDialogOpen={setTimestampDialogOpen}
      />
    </>
  );
}
