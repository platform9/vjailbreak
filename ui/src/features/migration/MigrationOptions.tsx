import {
  Box,
  Checkbox,
  FormControl,
  FormControlLabel,
  MenuItem,
  Select,
  styled,
  TextField,
} from "@mui/material"
import dayjs from "dayjs"
import { DesktopTimePicker } from "@mui/x-date-pickers/DesktopTimePicker"
import { LocalizationProvider } from "@mui/x-date-pickers/LocalizationProvider"
import { AdapterDayjs } from "@mui/x-date-pickers/AdapterDayjs"
import Step from "src/components/forms/Step"
import useParams from "src/hooks/useParams"

// Accordian Imports
import Accordion from "@mui/material/Accordion"
import AccordionSummary from "@mui/material/AccordionSummary"
import AccordionDetails from "@mui/material/AccordionDetails"
import ExpandMoreIcon from "@mui/icons-material/ExpandMore"
import { FormValues } from "./MigrationForm"

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

const CustomTextField = styled(TextField)({
  "& .MuiOutlinedInput-root": {
    height: "40px", // Adjust the overall container height
    fontFamily: "Monospace",
  },
})

const Dates = styled("div")(() => ({
  [`input`]: {
    padding: "8px 14px",
    width: "40px",
  },
}))

// Intefaces
interface MigrationOptionsPropsInterface {
  params: FormValues
  onChange: (key: string) => (value: string) => void
}

// Default state for checkboxes
const defaultValues = {
  dataCopyMethod: false,
  dataCopyTimeWindow: false,
  dataCopyStartTime: false,
  dataCopyEndTime: false,
  cutoverFromOriginalToMigratedVM: false,
  cutoverTimeWindow: false,
  cutoverStartTime: false,
  cutoverEndTime: false,
  cutoverCommand: false,
  preDataCopyWebHook: false,
  postDataCopyWebHook: false,
  preCutoverWebHook: false,
  postCutoverWebHook: false,
}

const DATA_COPY_METHODS = [
  { value: "hot", label: "Hot Copy" },
  { value: "cold", label: "Cold Copy" },
]

const PrePostWebHooksList = [
  { label: "Pre data-copy web hook", identifier: "preDataCopyWebHook" },
  { label: "Post data-copy web hook", identifier: "postDataCopyWebHook" },
  { label: "Pre cutover web hook", identifier: "preCutoverWebHook" },
  { label: "Post cutover web hook", identifier: "postCutoverWebHook" },
]

export default function MigrationOptions({
  params,
  onChange,
}: MigrationOptionsPropsInterface) {
  const { params: checkedParams, getParamsUpdater: updateCheckedParams } =
    useParams(defaultValues)

  console.log(params)

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
                      checked={checkedParams.dataCopyMethod}
                      onChange={(e) => {
                        updateCheckedParams("dataCopyMethod")(e.target.checked)
                      }}
                    />
                  }
                />
                <Select
                  labelId="source-item-label"
                  defaultValue="hot"
                  value={params.dataCopyMethod}
                  onChange={(e) => onChange("dataCopyMethod")(e.target.value)}
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
                    checked={checkedParams.dataCopyTimeWindow}
                    onChange={(e) => {
                      updateCheckedParams("dataCopyTimeWindow")(
                        e.target.checked
                      )
                    }}
                  />
                }
              />
              <Fields sx={{ ml: "32px" }}>
                <TimePicker
                  label="Start Time"
                  identifier="dataCopyStartTime"
                  params={params}
                  checkedParams={checkedParams}
                  updateCheckedParams={updateCheckedParams}
                  onChange={onChange}
                />
                <TimePicker
                  label="End Time"
                  identifier="dataCopyEndTime"
                  params={params}
                  checkedParams={checkedParams}
                  updateCheckedParams={updateCheckedParams}
                  onChange={onChange}
                />
              </Fields>
              <br />

              {/* Cutover settings*/}
              <FormControlLabel
                label="Cutover from original to migrated VM"
                control={
                  <Checkbox
                    checked={checkedParams.cutoverFromOriginalToMigratedVM}
                    onChange={(e) => {
                      updateCheckedParams("cutoverFromOriginalToMigratedVM")(
                        e.target.checked
                      )
                    }}
                  />
                }
              />
              <Box sx={{ ml: "32px" }}>
                <FormControlLabel
                  label="Only within time window"
                  control={
                    <Checkbox
                      checked={checkedParams.cutoverTimeWindow}
                      onChange={(e) => {
                        updateCheckedParams("cutoverTimeWindow")(
                          e.target.checked
                        )
                      }}
                    />
                  }
                />
                <Fields sx={{ ml: "32px" }}>
                  <TimePicker
                    label="Start Time"
                    identifier="cutoverStartTime"
                    params={params}
                    checkedParams={checkedParams}
                    updateCheckedParams={updateCheckedParams}
                    onChange={onChange}
                  />
                  <TimePicker
                    label="End Time"
                    identifier="cutoverEndTime"
                    params={params}
                    checkedParams={checkedParams}
                    updateCheckedParams={updateCheckedParams}
                    onChange={onChange}
                  />
                </Fields>

                <Fields sx={{ gridTemplateColumns: "1fr 1fr" }}>
                  <FormControlLabel
                    label="Only if this command succeeds in migrated VM"
                    control={
                      <Checkbox
                        checked={checkedParams.cutoverCommand}
                        onChange={(e) => {
                          updateCheckedParams("cutoverCommand")(
                            e.target.checked
                          )
                        }}
                      />
                    }
                  />
                  <CustomTextField
                    value={params?.cutoverCommand}
                    onChange={(e) =>
                      onChange("cutoverCommand")(String(e.target.value))
                    }
                  />
                </Fields>
              </Box>
              <br />

              {PrePostWebHooksList.map((hook) => (
                <Fields key={`${hook.label}-${hook.identifier}`}>
                  <PrePostWebHooks
                    label={hook.label}
                    identifier={hook.identifier}
                    params={params}
                    checkedParams={checkedParams}
                    updateCheckedParams={updateCheckedParams}
                    onChange={onChange}
                  />
                </Fields>
              ))}
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
  checkedParams,
  updateCheckedParams,
  onChange,
}) => {
  const value = params[identifier]
    ? dayjs(`1970-01-01T${params[identifier]}:00`)
    : dayjs(params[identifier])

  const handleTimeChange = (newValue: dayjs.Dayjs | null, identifier) => {
    const formattedTime = newValue?.format("HH:mm")
    onChange(identifier)(String(formattedTime))
  }

  return (
    <Dates>
      <FormControlLabel
        label={label}
        control={
          <Checkbox
            checked={checkedParams[identifier]}
            onChange={(e) => {
              updateCheckedParams(identifier)(e.target.checked)
            }}
          />
        }
      />
      <DesktopTimePicker
        ampm={false}
        defaultValue={dayjs()}
        value={value}
        format="HH:mm"
        onChange={(newValue: dayjs.Dayjs | null) =>
          handleTimeChange(newValue, identifier)
        }
      />
    </Dates>
  )
}

const PrePostWebHooks = ({
  label,
  identifier,
  params,
  onChange,
  checkedParams,
  updateCheckedParams,
}) => {
  return (
    <>
      <FormControlLabel
        label={label}
        control={
          <Checkbox
            checked={checkedParams[identifier]}
            onChange={(e) => {
              updateCheckedParams(identifier)(e.target.checked)
            }}
          />
        }
      />
      <CustomTextField
        value={params[identifier]}
        onChange={(e) => onChange(identifier)(String(e.target.value))}
      />
    </>
  )
}
