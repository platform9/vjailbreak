import { useCallback } from "react"
import {
  Box,
  Checkbox,
  FormControl,
  FormControlLabel,
  MenuItem,
  Select,
  styled,
} from "@mui/material"
import dayjs from "dayjs"
import { DateTimePicker } from "@mui/x-date-pickers/DateTimePicker"
import { LocalizationProvider } from "@mui/x-date-pickers/LocalizationProvider"
import { AdapterDayjs } from "@mui/x-date-pickers/AdapterDayjs"
import Step from "src/components/forms/Step"
import { FormValues, MigrationOptionsType } from "./MigrationForm"

// Accordian Imports
import Accordion from "@mui/material/Accordion"
import AccordionSummary from "@mui/material/AccordionSummary"
import AccordionDetails from "@mui/material/AccordionDetails"
import ExpandMoreIcon from "@mui/icons-material/ExpandMore"

// Styles
const FieldsContainer = styled("div")(({ theme }) => ({
  marginLeft: theme.spacing(4),
}))

const Fields = styled("div")(({ theme }) => ({
  display: "grid",
  gridTemplateColumns: "1fr 2fr",
  gridGap: "8px", // Adds spacing between the columns
  marginTop: theme.spacing(1),
}))

// const CustomTextField = styled(TextField)({
//   "& .MuiOutlinedInput-root": {
//     height: "40px", // Adjust the overall container height
//     fontFamily: "Monospace",
//   },
// })

const Dates = styled("div")(() => ({
  [`input`]: {
    padding: "8px 14px",
    width: "140px",
  },
}))

// Intefaces
interface MigrationOptionsPropsInterface {
  params: FormValues
  onChange: (key: string) => (value: unknown) => void
  migrationOptions: MigrationOptionsType
  updateMigrationOptions: (
    key: keyof MigrationOptionsType
  ) => (value: unknown) => void
}

const DATA_COPY_METHODS = [
  { value: "hot", label: "Hot Copy" },
  { value: "cold", label: "Cold Copy" },
]

// const PrePostWebHooksList = [
//   { label: "Pre data-copy web hook", identifier: "preDataCopyWebHook" },
//   { label: "Post data-copy web hook", identifier: "postDataCopyWebHook" },
//   { label: "Pre cutover web hook", identifier: "preCutoverWebHook" },
//   { label: "Post cutover web hook", identifier: "postCutoverWebHook" },
// ]

// TODO - Commented out the non-required field from the options for now
export default function MigrationOptions({
  params,
  onChange,
  migrationOptions,
  updateMigrationOptions,
}: MigrationOptionsPropsInterface) {
  console.log("Logs: ", migrationOptions, params)

  return (
    <LocalizationProvider dateAdapter={AdapterDayjs}>
      <Accordion
        sx={{
          boxShadow: "none", // Removes box shadow
          border: "none", // Removes border
          "&:before": {
            display: "none", // Removes the default divider line before the accordion
          },
        }}
        defaultExpanded
      >
        <AccordionSummary
          expandIcon={<ExpandMoreIcon />}
          aria-controls="panel2-content"
          id="panel2-header"
        >
          <Step
            stepNumber="4"
            label="Migration Options (Optional)"
            sx={{ mb: "0" }}
          />
        </AccordionSummary>
        <AccordionDetails>
          <FieldsContainer>
            <FormControl
              fullWidth
              size="small"
              disabled={DATA_COPY_METHODS.length === 0}
            >
              {/* Data Copy */}
              <Fields>
                <FormControlLabel
                  id="data-copy-method"
                  label="Data copy method"
                  control={
                    <Checkbox
                      checked={migrationOptions.dataCopyMethod}
                      onChange={(e) => {
                        updateMigrationOptions("dataCopyMethod")(
                          e.target.checked
                        )
                      }}
                    />
                  }
                />
                <Select
                  labelId="source-item-label"
                  value={params?.dataCopyMethod || "hot"}
                  onChange={(e) => {
                    onChange("dataCopyMethod")(e.target.value)
                    updateMigrationOptions("dataCopyMethod")(true)
                  }}
                >
                  {DATA_COPY_METHODS.map((item) => (
                    <MenuItem key={item.value} value={item.value}>
                      {item.label}
                    </MenuItem>
                  ))}
                </Select>
              </Fields>

              {/* Data Copy Time Window */}
              <FormControlLabel
                label="Only copy data within time window"
                control={
                  <Checkbox
                    checked={migrationOptions.dataCopyTimeWindow}
                    onChange={(e) => {
                      updateMigrationOptions("dataCopyTimeWindow")(
                        e.target.checked
                      )
                    }}
                  />
                }
              />
              <Fields sx={{ ml: "32px", gridTemplateColumns: "1fr 1fr" }}>
                <TimePicker
                  label="Start Time"
                  identifier="dataCopyStartTime"
                  params={params}
                  migrationOptions={migrationOptions}
                  updateMigrationOptions={updateMigrationOptions}
                  onChange={onChange}
                />
                {/* <TimePicker
                  label="End Time"
                  identifier="dataCopyEndTime"
                  params={params}
                  migrationOptions={migrationOptions}
                  updateMigrationOptions={updateMigrationOptions}
                  onChange={onChange}
                /> */}
              </Fields>

              {/* Cutover settings*/}
              <FormControlLabel
                label="Cutover from original to migrated VM"
                control={
                  <Checkbox
                    checked={migrationOptions.cutoverFromOriginalToMigratedVM}
                    onChange={(e) => {
                      updateMigrationOptions("cutoverFromOriginalToMigratedVM")(
                        e.target.checked
                      )
                    }}
                  />
                }
              />
              <Box sx={{ ml: "32px" }}>
                {/* <FormControlLabel
                  label="Only within time window"
                  control={
                    <Checkbox
                      checked={migrationOptions.cutoverTimeWindow}
                      onChange={(e) => {
                        updateMigrationOptions("cutoverTimeWindow")(
                          e.target.checked
                        )
                      }}
                    />
                  }
                /> */}
                <Fields sx={{ gridTemplateColumns: "1fr 1fr" }}>
                  <TimePicker
                    label="Start Time"
                    identifier="cutoverStartTime"
                    params={params}
                    migrationOptions={migrationOptions}
                    updateMigrationOptions={updateMigrationOptions}
                    onChange={onChange}
                  />
                  <TimePicker
                    label="End Time"
                    identifier="cutoverEndTime"
                    params={params}
                    migrationOptions={migrationOptions}
                    updateMigrationOptions={updateMigrationOptions}
                    onChange={onChange}
                  />
                </Fields>

                {/* <Fields sx={{ gridTemplateColumns: "1fr 1fr" }}>
                  <FormControlLabel
                    label="Only if this command succeeds in migrated VM"
                    control={
                      <Checkbox
                        checked={migrationOptions.cutoverCommand}
                        onChange={(e) => {
                          updateMigrationOptions("cutoverCommand")(
                            e.target.checked
                          )
                        }}
                      />
                    }
                  />
                  <CustomTextField
                    value={params?.cutoverCommand || ""}
                    onChange={(e) =>
                      onChange("cutoverCommand")(String(e.target.value))
                    }
                  />
                </Fields> */}
              </Box>

              {/* Retry on failure */}
              <FormControlLabel
                label="Retry on failure"
                control={
                  <Checkbox
                    checked={params?.retryOnFailure || false}
                    onChange={(e) => {
                      onChange("retryOnFailure")(e.target.checked)
                    }}
                  />
                }
              />

              {/* Pre and Post Web Hooks */}
              {/* {PrePostWebHooksList.map((hook) => (
                <Fields key={`${hook.label}-${hook.identifier}`}>
                  <PrePostWebHooks
                    label={hook.label}
                    identifier={hook.identifier}
                    params={params}
                    migrationOptions={migrationOptions}
                    updateMigrationOptions={updateMigrationOptions}
                    onChange={onChange}
                  />
                </Fields>
              ))} */}
            </FormControl>
          </FieldsContainer>
        </AccordionDetails>
      </Accordion>
    </LocalizationProvider>
  )
}

const TimePicker = ({
  label,
  identifier,
  params,
  migrationOptions,
  updateMigrationOptions,
  onChange,
}) => {
  const value = params?.[identifier] ? dayjs(params?.[identifier]) : dayjs()

  const handleTimeChange = useCallback(
    (newValue: dayjs.Dayjs | null, identifier) => {
      const formattedTime = newValue?.toISOString()
      onChange(identifier)(String(formattedTime))
      updateMigrationOptions(identifier)(true)
    },
    [onChange]
  )

  return (
    <Dates>
      <FormControlLabel
        label={label}
        control={
          <Checkbox
            checked={migrationOptions?.[identifier]}
            onChange={(e) => {
              updateMigrationOptions(identifier)(e.target.checked)
            }}
          />
        }
      />
      <DateTimePicker
        ampm={false}
        defaultValue={dayjs()}
        value={value}
        onChange={(newValue: dayjs.Dayjs | null) =>
          handleTimeChange(newValue, identifier)
        }
      />
    </Dates>
  )
}

// const PrePostWebHooks = ({
//   label,
//   identifier,
//   params,
//   onChange,
//   migrationOptions,
//   updateMigrationOptions,
// }) => {
//   return (
//     <>
//       <FormControlLabel
//         label={label}
//         control={
//           <Checkbox
//             checked={migrationOptions?.[identifier]}
//             onChange={(e) => {
//               updateMigrationOptions(identifier)(e.target.checked)
//             }}
//           />
//         }
//       />
//       <CustomTextField
//         value={params?.[identifier] || ""}
//         onChange={(e) => onChange(identifier)(String(e.target.value))}
//       />
//     </>
//   )
// }
